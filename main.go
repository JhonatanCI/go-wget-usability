package main

import (
	"fmt"
	"os"
	"time"
    "wget/database"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"

	"os/exec"
	"path/filepath"
	"math/rand"
	"strconv"

	//"yourmodule/database" // reemplaza esto por el path correcto a tu package database
)

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


var downloadQueue = make(chan DownloadRequest, 20)
var isRunning bool

/*
if !isRunning {
					isRunning = true
					go func() {
						defer func() { isRunning = false }()      // Al terminar, liberamos el flag
						doc_manager.TosendcorrespondenciaVERIFY() // Llama a la función del otro modelo
					}()
				}
*/

func main() {
	e := echo.New()

	if err := godotenv.Load("./.env"); err != nil {
		panic(err)
	}

	// 🧠 Worker principal que ejecuta las tareas
	go func() {
		for req := range downloadQueue {
			start := time.Now()
			fmt.Printf("📥 Procesando tarea ID=%s tipo=%s\n", req.ID, req.Type)

			err := processDownload(req)
			if err != nil {
				fmt.Printf("❌ Error en tarea %s: %v\n", req.ID, err)
				updateTaskState(req.ID, "002")
			} else {
				fmt.Printf("✅ Completado ID=%s en %s\n", req.ID, time.Since(start))
				updateTaskState(req.ID, "003")
			}
		}
	}()

	// ⏰ Ticker que revisa la BD cada minuto
	ticker := time.NewTicker(time.Minute)
    go func() {
        for range ticker.C {
            tasks, err := getPendingTasks()
            if err != nil {
                fmt.Println("Error al obtener tareas:", err)
                continue
            }

            for _, task := range tasks {
                if err := updateTaskState(task.ID, "001"); err != nil {
                    fmt.Println("Error al actualizar estado:", err)
                    continue
                }

                select {
                case downloadQueue <- task:
                    fmt.Println("✅ Tarea encolada:", task.ID)
                default:
                    fmt.Println("⚠️ Cola llena. No se encoló:", task.ID)
                }
            }
        }
    }()


	// 🧾 Endpoint para encolar tareas manualmente
	e.POST("/download", func(c echo.Context) error {
		var req DownloadRequest
		if err := c.Bind(&req); err != nil {
			return c.String(400, "JSON inválido: "+err.Error())
		}
		if err := insertTaskToDB(req); err != nil {
			return c.String(500, "Error al guardar en BD: "+err.Error())
		}

		select {
		case downloadQueue <- req:
			_ = updateTaskState(req.ID, "001")
			return c.String(200, "Tarea encolada correctamente.")
		default:
			return c.String(503, "Cola llena, intenta luego.")
		}
	})
	e.PUT("/update", func(c echo.Context) error {
		var req DownloadRequest
		if err := c.Bind(&req); err != nil {
			return c.String(400, "JSON inválido: "+err.Error())
		}

		if err := updateTaskByID(req, "000"); err != nil {
			return c.String(500, "Error actualizando tarea: "+err.Error())
		}

		return c.String(200, "Tarea actualizada correctamente.")
	})


	e.Logger.Fatal(e.Start(":8080"))
    
}

func updateTaskByID(task DownloadRequest, estado string) error {
	conn, err := database.Connect()
	if err != nil {
		return err
	}
	defer database.Close(conn)

	_, err = database.Exec(conn,
		`UPDATE tareas SET estado = $1, name_descomprimido = $2, download = $3, route_destino = $4, route_origen = $5, service = $6, control_file = $7 WHERE id = $8`,
		estado, task.NameDescomprimido, task.Download, task.RouteDestino, task.RouteOrigen, task.Service, task.ControlFile, task.ID)
	return err
}

func processDownload(req DownloadRequest) error {
	updateDir, _, err := createUniqueTempDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(updateDir)

	switch req.Type {
	case "backend":
		fileName := req.NameDescomprimido // "back"
		if err := os.MkdirAll(updateDir, 0755); err != nil {
			return fmt.Errorf("crear carpeta update: %w", err)
		}
		// Descargar el archivo
		if err := download(req.Download, fileName, updateDir); err != nil {
			return fmt.Errorf("descargar: %w", err)
		}
		// Mover/reemplazar el archivo en destino
		destPath := filepath.Join(req.RouteDestino, "")
		if err := moveAndReplace(fileName, req.RouteDestino, updateDir); err != nil {
			return fmt.Errorf("mover/reemplazar: %w", err)
		}
		fmt.Printf("🔐 Aplicando permisos a: %s\n", destPath)
		if err := setPermissions(destPath, "777"); err != nil {
			return fmt.Errorf("permisos: %w", err)
		}
		serviceName := req.Service
		if err := applyService(serviceName); err != nil {
			return fmt.Errorf("reiniciar servicio: %w", err)
		}
		if err := createFile(req.ControlFile); err != nil {
			return fmt.Errorf("crear archivo de control: %w", err)
		}
	case "public":
		zipFile := req.NameDescomprimido + ".zip"
		if err := os.MkdirAll(updateDir, 0755); err != nil {
			return fmt.Errorf("crear carpeta update: %w", err)
		}
		if err := downloadAndUnzip(req.Download, zipFile, updateDir); err != nil {
			return fmt.Errorf("descargar y descomprimir: %w", err)
		}
		srcPath := filepath.Join(updateDir, "/*")
		destPath := req.RouteDestino
		sudoMkdirAll(destPath);
		if err := exec.Command("bash", "-c", "sudo cp -R "+srcPath+" "+destPath).Run(); err != nil {
			return fmt.Errorf("copiar archivos: %w", err)
		}
		if err := setPermissions(destPath, "777"); err != nil {
			return fmt.Errorf("permisos: %w", err)
		}

	case "resources":
		executableName := req.NameDescomprimido
		if err := os.MkdirAll(updateDir, 0755); err != nil {
			return fmt.Errorf("crear carpeta update: %w", err)
		}
		if err := download(req.Download, executableName, updateDir); err != nil {
			return fmt.Errorf("descargar ejecutable: %w", err)
		}
		if err := moveAndReplace(executableName, req.RouteDestino, updateDir); err != nil {
			return fmt.Errorf("mover ejecutable: %w", err)
		}
		destPath := filepath.Join(req.RouteDestino, executableName)
		if err := setPermissions(destPath, "777"); err != nil {
			return fmt.Errorf("permisos: %w", err)
		}

	case "new_folder":
		if req.RouteDestino == "" || req.NameDescomprimido == "" {
			return fmt.Errorf("faltan datos para new_folder")
		}
		fullPath := filepath.Join(req.RouteDestino, req.NameDescomprimido)
		if err := sudoMkdirAll(fullPath); err != nil {
			return fmt.Errorf("crear carpeta: %w", err)
		}
		if err := setPermissions(fullPath, "777"); err != nil {
			return fmt.Errorf("permisos: %w", err)
		}

	case "replace_folder":
		if req.RouteOrigen == "" || req.Download == "" || req.NameDescomprimido == "" {
			return fmt.Errorf("faltan datos para replace_folder")
		}
		if _, err := os.Stat(req.RouteOrigen); err == nil {
			oldFolderPath := req.RouteOrigen + "_old_" + time.Now().Format("20060102150405")
			if err := exec.Command("sudo", "mv", req.RouteOrigen, oldFolderPath).Run(); err != nil {
				return fmt.Errorf("renombrar carpeta original: %w", err)
			}
		}
		zipFile := req.NameDescomprimido + ".zip"
		if err := os.MkdirAll(updateDir, 0755); err != nil {
			return fmt.Errorf("crear carpeta update: %w", err)
		}
		if err := downloadAndUnzip(req.Download, zipFile, updateDir); err != nil {
			return fmt.Errorf("descargar y descomprimir: %w", err)
		}
		sudoMkdirAll(req.RouteDestino);
		cpCmd := exec.Command("bash", "-c", "sudo cp -R "+filepath.Join(updateDir, "*")+" "+req.RouteDestino)
		if out, err := cpCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("copiar archivos al destino: %w - %s", err, string(out))
		}
		if err := setPermissions(req.RouteDestino, "777"); err != nil {
			return fmt.Errorf("permisos: %w", err)
		}

	case "reset":
		if req.Service == "" {
			return fmt.Errorf("faltan datos para reset")
		}
		if err := applyService(req.Service); err != nil {
			return fmt.Errorf("reiniciar servicio: %w", err)
		}

	default:
		return fmt.Errorf("tipo no soportado: %s", req.Type)
	}

	return nil
}


func downloadAndUnzip(url, zipFile, dir string) error {
	// Descargar el archivo zip en la carpeta 'update'
	updateDir := dir
	filePath := filepath.Join(updateDir, zipFile)
	if err := exec.Command("wget", "-O", filePath, url).Run(); err != nil {
		return err // Si wget falla, retornamos el error
	}
	fmt.Println("Archivos descargados en:", updateDir)
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

	// Solo crea el destino si NO existe
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "mkdir", "-p", dest)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("error al crear carpeta con sudo: %v - %s", err, string(out))
		}
	}

	// Realiza el movimiento forzado
	return exec.Command("sudo", "mv", "-f", srcPath, dest).Run()
}


func setPermissions(path, perms string) error {
    if _, err := os.Stat(path); err != nil {
        return fmt.Errorf("la ruta %s no existe: %v", path, err)
    }
    fmt.Printf("Cambiando permisos: sudo chmod -R %s %s\n", perms, path)
    cmd := exec.Command("sudo", "chmod", "-R", perms, path)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error al cambiar permisos en %s: %v - %s", path, err, string(out))
    }
    return nil
}


func applyService(service string) error {
    cmd := exec.Command("sudo", "systemctl", "restart", service, ".service")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("error al reiniciar servicio %s: %v - %s", service, err, string(out))
    }
    return nil
}

func createFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func sudoMkdirAll(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cmd := exec.Command("sudo", "mkdir", "-p", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("error al crear carpeta con sudo: %v - %s", err, string(out))
		}
	}
	return nil
}


func getPendingTasks() ([]DownloadRequest, error) {
	conn, err := database.Connect()
	if err != nil {
		return nil, err
	}
	defer database.Close(conn)

	query := `SELECT id, type, name_descomprimido, download, route_destino, route_origen, service, control_file FROM tareas WHERE estado = '000' LIMIT 10`
	rows, err := database.Query(conn, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tareas []DownloadRequest
	for rows.Next() {
		var t DownloadRequest
        fmt.Printf("📝 Cargando tarea desde BD: %+v\n", t)
		if err := rows.Scan(&t.ID, &t.Type, &t.NameDescomprimido, &t.Download, &t.RouteDestino, &t.RouteOrigen, &t.Service, &t.ControlFile); err != nil {
			continue
		}
		tareas = append(tareas, t)
	}
	return tareas, nil
}

func updateTaskState(id string, estado string) error {
	conn, err := database.Connect()
	if err != nil {
		return err
	}
	defer database.Close(conn)

	_, err = database.Exec(conn, `UPDATE tareas SET estado = $1 WHERE id = $2`, estado, id)
    fmt.Printf("🔄 Estado actualizado para tarea %s → %s\n", id, estado)
	return err
}

func insertTaskToDB(task DownloadRequest) error {
	conn, err := database.Connect()
	if err != nil {
		return err
	}
	defer database.Close(conn)

	_, err = database.Exec(conn, `
		INSERT INTO tareas (id, type, name_descomprimido, download, route_destino, route_origen, service, control_file, estado)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '000')`,
		task.ID, task.Type, task.NameDescomprimido, task.Download,
		task.RouteDestino, task.RouteOrigen, task.Service, task.ControlFile)
	return err
}

func createUniqueTempDir() (string, string, error) {
	base := "update"

	if err := os.MkdirAll(base, 0777); err != nil {
		return "", "", fmt.Errorf("crear carpeta base 'update': %w", err)
	}

	randPart := strconv.Itoa(rand.Intn(1000000))
	timestamp := time.Now().Format("20060102150405")
	subFolder := randPart + "_" + timestamp
	fullPath := filepath.Join(base, subFolder)

	if err := os.MkdirAll(fullPath, 0777); err != nil {
		return "", "", fmt.Errorf("crear subcarpeta temporal: %w", err)
	}

	return fullPath, subFolder, nil // ← devuelves la ruta completa y el nombre
}



/*
update task with id *****
update db with endppotin********
create this <<<<<<<<<<


var process = strconv.Itoa(rand.Int()) + time.Now().Format("20060102170604")


			carpetatemporal := "./update/" + process
			if _, err := os.Stat(carpetatemporal); os.IsNotExist(err) {
				os.MkdirAll(carpetatemporal, 0777)
			}

			//se hace el  proceso


			os.RemoveAll(carpetatemporal)*/