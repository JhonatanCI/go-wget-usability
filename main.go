package main

import (
    "net/http"
    "os/exec"
    "github.com/labstack/echo/v4"
)

func main() {
    e := echo.New()

    e.GET("/download", func(c echo.Context) error {
        t := c.QueryParam("type")
        url := "https://abkaerp.com/documentos/filedeskv2/updatev2/new/v5/public_https.zip"
        zipFile := "public_https.zip"

        // Descargar el archivo
        cmdWget := exec.Command("wget", url)
        if err := cmdWget.Run(); err != nil {
            return c.String(http.StatusInternalServerError, "Error al descargar: "+err.Error())
        }

        // Descomprimir el archivo
        cmdUnzip := exec.Command("unzip", "-o", zipFile)
        if err := cmdUnzip.Run(); err != nil {
            return c.String(http.StatusInternalServerError, "Error al descomprimir: "+err.Error())
        }

        if t == "move" {
            // Crear carpeta nivel2 si no existe y mover archivos
            cmdMkdir := exec.Command("mkdir", "-p", "nivel2")
            if err := cmdMkdir.Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al crear carpeta: "+err.Error())
            }
            cmdMv := exec.Command("mv", "-f", "*", "nivel2/")
            if err := cmdMv.Run(); err != nil {
                return c.String(http.StatusInternalServerError, "Error al mover archivos: "+err.Error())
            }
        }

        return c.String(http.StatusOK, "Proceso completado para type: "+t)
    })

    e.Logger.Fatal(e.Start(":8080"))
}