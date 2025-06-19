package main

import (
    "net/http"
    "os"
    "os/exec"
    "github.com/labstack/echo/v4"
)

// Estructura para recibir el JSON
type DownloadRequest struct {
    Type string `json:"type"`
}

func main() {
    e := echo.New()

    e.POST("/download", func(c echo.Context) error {
        var req DownloadRequest
        if err := c.Bind(&req); err != nil {
            return c.String(http.StatusBadRequest, "JSON inválido: "+err.Error())
        }

        t := req.Type
        url := "https://abkaerp.com/documentos/filedeskv2/updatev2/new/v5/public_https.zip"
        zipFile := "public_https.zip"
		unzippedFolder := "public_https"

        switch t {
        case "normal":
            if err := downloadAndUnzip(url, zipFile); err != nil {
                return c.String(http.StatusInternalServerError, "Error al descargar y descomprimir: "+err.Error())
            }
            os.Remove(zipFile)
            return c.String(http.StatusOK, "Proceso completado para type: "+t)

        case "move":
            if err := exec.Command("mkdir", "-p", "nivel2").Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta: "+err.Error())
            }
            // Mover la carpeta descomprimida a nivel2
            if err := exec.Command("mv", "-f", unzippedFolder, "nivel2/").Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al mover carpeta: "+err.Error())
            }
            os.Remove(zipFile)
            return c.String(http.StatusOK, "Proceso completado para type: "+t)

        default:
            return c.String(http.StatusBadRequest, "Tipo no soportado")
        }
    })

    e.Logger.Fatal(e.Start(":8080"))
}

func downloadAndUnzip(url, zipFile string) error {
    if err := exec.Command("wget", "-O", zipFile, url).Run(); err != nil {
        return err
    }
    if err := exec.Command("unzip", "-o", zipFile).Run(); err != nil {
        return err
    }
    return nil
}