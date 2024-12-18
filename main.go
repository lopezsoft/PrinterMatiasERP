package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	port := getServerPort()
	http.HandleFunc("/print", printHandler)
	http.HandleFunc("/open-box", openDrawerHandler)
	http.HandleFunc("/list-printers", listPrintersHandler)
	log.Printf("El servidor ha iniciado en :%d", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatalf("Fallo al iniciar el servidor: %v", err)
	}
}

func getServerPort() int {
	defaultPort := 8080
	if port, ok := os.LookupEnv("PORT"); ok {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}
	return defaultPort
}

func listPrintersHandler(w http.ResponseWriter, r *http.Request) {
	printers, err := listPrinters()
	if err != nil {
		log.Printf("Error al listar las impresoras: %v", err)
		http.Error(w, fmt.Sprintf("Error al listar las impresoras: %v", err), http.StatusInternalServerError)
		return
	}
	jsonData, err := json.Marshal(printers)
	if err != nil {
		log.Printf("Error al codificar la lista de impresoras en JSON: %v", err)
		http.Error(w, "Error al procesar datos", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func listPrinters() ([]string, error) {
	cmd := exec.Command("powershell", "Get-Printer | Select -ExpandProperty Name")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	printers := strings.Split(strings.TrimSpace(out.String()), "\n")
	return printers, nil
}

func openDrawerHandler(w http.ResponseWriter, r *http.Request) {
	printerName := r.URL.Query().Get("printer")
	if printerName == "" {
		http.Error(w, "No se especificó la impresora", http.StatusBadRequest)
		return
	}

	err := openDrawer(printerName)
	if err != nil {
		log.Printf("Error al abrir el cajón: %v", err)
		http.Error(w, fmt.Sprintf("Error al abrir el cajón: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Cajón abierto exitosamente."))
}

func openDrawer(printerName string) error {
	cmd := exec.Command("print", "/D:"+printerName, "./drawer_open_command.txt")
	return cmd.Run()
}

func printHandler(w http.ResponseWriter, r *http.Request) {
	urlParam := r.URL.Query().Get("url")
	printer := r.URL.Query().Get("printer")

	if urlParam == "" || printer == "" {
		http.Error(w, "URL o impresora no especificados", http.StatusBadRequest)
		return
	}

	if _, err := url.ParseRequestURI(urlParam); err != nil {
		http.Error(w, "URL inválida", http.StatusBadRequest)
		return
	}

	filePath, err := downloadFile(urlParam)
	if err != nil {
		log.Printf("Error al descargar el archivo: %v", err)
		http.Error(w, fmt.Sprintf("Error al descargar el archivo: %v", err), http.StatusInternalServerError)
		return
	}

	defer os.Remove(filePath)

	if err := printFile(filePath, printer); err != nil {
		log.Printf("Error al imprimir el archivo: %v", err)
		http.Error(w, fmt.Sprintf("Error al imprimir el archivo: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write([]byte("PDF enviado a la impresora exitosamente."))
}

func downloadFile(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servidor retornó un estado no OK: %d %s", resp.StatusCode, resp.Status)
	}

	tempFile, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func printFile(filePath, printer string) error {
	cmd := exec.Command("./PDFtoPrinter.exe", filePath, printer)
	cmd.Stderr = os.Stderr // Captura y muestra errores de impresión
	return cmd.Run()
}
