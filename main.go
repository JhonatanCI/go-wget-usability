package main

import (
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "github.com/labstack/echo/v4"
)

// Estructura para recibir el JSON
type DownloadRequest struct {
    ID                string `json:"id"`
    Type              string `json:"type"`
    NameDescomprimido string `json:"name_descomprimido"`
    Download          string `json:"download"`
    RouteDestino      string `json:"route_destino"`
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

        switch req.Type {
        case "backend":
            updateDir := "update"
            zipFile := req.NameDescomprimido + ".zip"
            unzippedFolder := req.NameDescomprimido

            // Crear carpeta update si no existe
            if err := os.MkdirAll(updateDir, 0755); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta update: "+err.Error())
            }

            // Descargar y descomprimir
            if err := downloadAndUnzip(req.Download, zipFile, updateDir); err != nil {
                return c.String(http.StatusInternalServerError, "Error al descargar y descomprimir: "+err.Error())
            }

            // Mover/reemplazar en destino
            destPath := filepath.Join(req.RouteDestino, unzippedFolder)
            if err := moveAndReplace(unzippedFolder, req.RouteDestino); err != nil {
                return c.String(http.StatusInternalServerError, "Error al mover/reemplazar: "+err.Error())
            }

            // Dar permisos
            if err := setPermissions(destPath, "777"); err != nil {
                return c.String(http.StatusInternalServerError, "Error al dar permisos: "+err.Error())
            }

            // Reiniciar servicio
            serviceName := "filedesk-cloud." + req.Service
            if err := restartService(serviceName); err != nil {
                return c.String(http.StatusInternalServerError, "Error al reiniciar el servicio: "+err.Error())
            }

            // Crear archivo de control en la ruta recibida por JSON
            if err := createFile(req.ControlFile); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear archivo de control: "+err.Error())
            }

            return c.String(http.StatusOK, "Proceso backend completado correctamente.")

        default:
            return c.String(http.StatusBadRequest, "Tipo no soportado")
        }
    })

    e.Logger.Fatal(e.Start(":8080"))
}

func downloadAndUnzip(url, zipFile, dir string) error {
    // Descargar el archivo zip en la carpeta update
    updateDir := dir
	filePath := filepath.Join(updateDir, zipFile)
	if err := exec.Command("wget", "-O", filePath, url).Run(); err != nil {
		return err
	}
    // Descomprimir el archivo dentro de la carpeta update
    cmd := exec.Command("unzip", "-o", filePath, "-d", dir)
    if err := cmd.Run(); err != nil {
        return err
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

func moveAndReplace(folder, dest string) error {
    return exec.Command("mv", "-f", folder, dest).Run()
}

func setPermissions(path, perms string) error {
    return exec.Command("sudo", "chmod", "-R", perms, path).Run()
}

func restartService(service string) error {
    return exec.Command("sudo", "systemctl", "restart", service).Run()
}

func createFile(path string) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    return f.Close()
}