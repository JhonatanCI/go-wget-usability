package main

import (
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "github.com/labstack/echo/v4"
	"time"
)

// Estructura para recibir el JSON
type DownloadRequest struct {
    ID                string `json:"id"`
    Type              string `json:"type"`
    NameDescomprimido string `json:"name_descomprimido"`
    Download          string `json:"download"`
    RouteDestino      string `json:"route_destino"`
	RouteOrigen       string `json:"route_origen"`
    Service           string `json:"service"`
    ControlFile       string `json:"control_file"`
}

func main() {
    e := echo.New()

    e.POST("/download", func(c echo.Context) error {
        var req DownloadRequest
        if err := c.Bind(&req); err != nil {
            return c.String(http.StatusBadRequest, "JSON inválido: "+err.Error())
        }
		
		updateDir := "update"
        
		switch req.Type {
        case "backend":
            
            zipFile := req.NameDescomprimido
            unzippedFolder := req.NameDescomprimido

            // Crear carpeta update si no existe
            if err := os.MkdirAll(updateDir, 0755); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta update: "+err.Error())
            }

            // Descargar y descomprimir
            if err := download(req.Download, zipFile, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al descargar: "+err.Error())
            }

            // Mover/reemplazar en destino
            destPath := filepath.Join(req.RouteDestino, unzippedFolder)
            if err := moveAndReplace(unzippedFolder, req.RouteDestino, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al mover/reemplazar: "+err.Error())
            }

            // Dar permisos
            if err := setPermissions(destPath, "777"); err != nil {
                return c.String(http.StatusInternalServerError, "Error al dar permisos: "+err.Error())
            }

            // Reiniciar servicio
            serviceName := "filedesk-cloud." + req.Service
            if err := applyService(serviceName); err != nil {
                return c.String(http.StatusInternalServerError, "Error al reiniciar el servicio: "+err.Error())
            }

            // Crear archivo de control en la ruta recibida por JSON
            if err := createFile(req.ControlFile); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear archivo de control: "+err.Error())
            }

            return c.String(http.StatusOK, "Proceso backend completado correctamente.")
		
		case "public":
            zipFile := req.NameDescomprimido + ".zip"
            unzippedFolder := req.NameDescomprimido 

            if err := os.MkdirAll(updateDir, 0755); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta update: "+err.Error())
            }

            // Descargar y descomprimir el zip en update
            if err := downloadAndUnzip(req.Download, zipFile, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al descargar y descomprimir: "+err.Error())
            }

            // Copiar el contenido descomprimido a la ruta destino
            srcPath := filepath.Join(updateDir, unzippedFolder) + "/*"
            destPath := req.RouteDestino
            if err := exec.Command("sudo", "mkdir", "-p", destPath).Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear ruta destino con sudo: "+err.Error())
            }

            // sudo cp -R update/public_https/* /usr/bin/fd_cloud/public/
            if err := exec.Command("bash", "-c", "sudo cp -R "+srcPath+" "+destPath).Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al copiar archivos: "+err.Error())
            }

            // Dar permisos a la carpeta destino
            if err := setPermissions(destPath, "777"); err != nil {
                return c.String(http.StatusInternalServerError, "Error al dar permisos: "+err.Error())
            }

            return c.String(http.StatusOK, "Proceso public completado correctamente.")

		case "resources":
            // El nombre del archivo ejecutable que vamos a manejar.
            executableName := req.NameDescomprimido 

            // 1. Asegurarse que el directorio 'update' existe.
            if err := os.MkdirAll(updateDir, 0755); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta update: "+err.Error())
            }

            // 2. Descargar el archivo ejecutable en la carpeta 'update'.
            if err := download(req.Download, executableName, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al descargar el ejecutable: "+err.Error())
            }

            // 3. Mover y reemplazar el ejecutable en su destino final.
            if err := moveAndReplace(executableName, req.RouteDestino, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al mover el ejecutable: "+err.Error())
            }

            // 4. Dar permisos al archivo ejecutable en su nueva ubicación.
            destPath := filepath.Join(req.RouteDestino, executableName)
            if err := setPermissions(destPath, "777"); err != nil {
                return c.String(http.StatusInternalServerError, "Error al dar permisos al ejecutable: "+err.Error())
            }

            return c.String(http.StatusOK, "Proceso 'resources' completado correctamente.")

		case "new_folder":
			if req.RouteDestino == "" || req.NameDescomprimido == "" {
				return c.String(http.StatusBadRequest, "Para 'crear_carpeta', se requieren 'route_destino' y 'name_descomprimido'")
			}
			fullPath := filepath.Join(req.RouteDestino, req.NameDescomprimido)
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return c.String(http.StatusInternalServerError, "Error al crear la carpeta: "+err.Error())
			}
			if err := setPermissions(fullPath, "777"); err != nil {
				return c.String(http.StatusInternalServerError, "Error al dar permisos a la carpeta: "+err.Error())
			}
			return c.String(http.StatusOK, "Carpeta creada y con permisos en: "+fullPath)

		case "replace_folder":
			if req.RouteOrigen == "" || req.Download == "" || req.NameDescomprimido == "" {
				return c.String(http.StatusBadRequest, "Para 'reemplazar_carpeta' se requieren 'route_origen', 'download' y 'name_descomprimido'")
			}
			if _, err := os.Stat(req.RouteOrigen); err == nil {
				oldFolderPath := req.RouteOrigen + "_old_" + time.Now().Format("20060102150405")
				if err := exec.Command("sudo", "mv", req.RouteOrigen, oldFolderPath).Run(); err != nil {
					return c.String(http.StatusInternalServerError, "Error al renombrar carpeta original: "+err.Error())
				}
			}
			zipFile := req.NameDescomprimido + ".zip"
			if err := os.MkdirAll(updateDir, 0755); err != nil {
				return c.String(http.StatusInternalServerError, "Error al crear carpeta update: "+err.Error())
			}
			if err := downloadAndUnzip(req.Download, zipFile, updateDir); err != nil {
				return c.String(http.StatusInternalServerError, "Error al descargar y descomprimir: "+err.Error())
			}
			srcPath := filepath.Join(updateDir, req.NameDescomprimido)
			destPath := filepath.Join(req.RouteDestino, req.NameDescomprimido)
			if err := exec.Command("sudo", "mv", srcPath, req.RouteDestino).Run(); err != nil {
				return c.String(http.StatusInternalServerError, "Error al mover la nueva carpeta a su destino: "+err.Error())
			}
			if err := setPermissions(destPath, "777"); err != nil {
				return c.String(http.StatusInternalServerError, "Error al dar permisos a la nueva carpeta: "+err.Error())
			}
			os.Remove(filepath.Join(updateDir, zipFile))
			return c.String(http.StatusOK, "Carpeta reemplazada correctamente en: "+destPath)

		case "reset":
			if req.Service == "" {
				return c.String(http.StatusBadRequest, "El campo 'service' es requerido para reiniciar un servicio.")
			}
			
			if err := applyService(req.Service); err != nil {
				return c.String(http.StatusInternalServerError, "Error al reiniciar el servicio: "+err.Error())
			}
			return c.String(http.StatusOK, "Servicio '"+req.Service+"' reiniciado correctamente.")

		
        default:
            return c.String(http.StatusBadRequest, "Tipo no soportado")
        }
    })

    e.Logger.Fatal(e.Start(":8080"))
}


func downloadAndUnzip(url, zipFile, dir string) error {
	// Descargar el archivo zip en la carpeta 'update'
	updateDir := dir
	filePath := filepath.Join(updateDir, zipFile)
	if err := exec.Command("wget", "-O", filePath, url).Run(); err != nil {
		return err // Si wget falla, retornamos el error
	}

	cmd := exec.Command("unzip", "-o", zipFile)

	cmd.Dir = dir // dir es "update"

	// Ejecutamos el comando 'unzip' desde la carpeta 'update'
	if err := cmd.Run(); err != nil {
		return err // Si unzip falla, retornamos el error
	}

	return nil
}
func download(url, file, dir string) error {
    // Descargar el archivo zip en la carpeta update
    updateDir := dir
	filePath := filepath.Join(updateDir, file)
	if err := exec.Command("wget", "-O", filePath, url).Run(); err != nil {
		return err
	}
		return nil
}

func moveAndReplace(folder, dest string, dir string) error {
    srcPath := filepath.Join(dir, folder)
    // Crea la ruta destino si no existe
    if err := os.MkdirAll(dest, 0755); err != nil {
        return err
    }
    return exec.Command("sudo","mv", "-f", srcPath, dest).Run()
}

func setPermissions(path, perms string) error {
    return exec.Command("sudo", "chmod", "-R", perms, path).Run()
}

func applyService(service string) error {
    return exec.Command("sudo", "systemctl", "restart", service).Run()
}

func createFile(path string) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    return f.Close()
}