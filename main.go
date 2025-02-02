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
	"syscall" // Importa syscall para configurar SysProcAttr
	"time"

	"github.com/rs/cors"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ============================
// Configuración del Servidor
// ============================

// Config almacena las configuraciones del servidor y herramientas externas
type Config struct {
	Port              int
	PDFPrinterPath    string
	DrawerCommandPath string
	TLSCertPath       string
	TLSKeyPath        string
	AllowedOrigins    []string
	LogFile           string
	LogMaxSize        int
	LogMaxBackups     int
	LogMaxAge         int
	LogCompress       bool
	HTTPReadTimeout   int
	HTTPWriteTimeout  int
	HTTPIdleTimeout   int
}

// LoadConfig carga la configuración desde variables de entorno o valores por defecto
func LoadConfig() Config {
	return Config{
		Port:              getEnvAsInt("PORT", 8080),
		PDFPrinterPath:    getEnv("PDF_PRINTER_PATH", "./PDFtoPrinter.exe"),
		DrawerCommandPath: getEnv("DRAWER_COMMAND_PATH", "./drawer_open_command.txt"),
		TLSCertPath:       getEnv("TLS_CERT_PATH", ""),
		TLSKeyPath:        getEnv("TLS_KEY_PATH", ""),
		AllowedOrigins:    getEnvAsSlice("ALLOWED_ORIGINS", "*"),
		LogFile:           getEnv("LOG_FILE", "app.log"),
		LogMaxSize:        getEnvAsInt("LOG_MAX_SIZE_MB", 10),
		LogMaxBackups:     getEnvAsInt("LOG_MAX_BACKUPS", 3),
		LogMaxAge:         getEnvAsInt("LOG_MAX_AGE_DAYS", 28),
		LogCompress:       getEnvAsBool("LOG_COMPRESS", true),
		HTTPReadTimeout:   getEnvAsInt("HTTP_READ_TIMEOUT", 15),
		HTTPWriteTimeout:  getEnvAsInt("HTTP_WRITE_TIMEOUT", 15),
		HTTPIdleTimeout:   getEnvAsInt("HTTP_IDLE_TIMEOUT", 60),
	}
}

// Funciones auxiliares para obtener variables de entorno con valores por defecto
func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.ParseBool(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}

func getEnvAsSlice(key string, defaultVal string) []string {
	if val, ok := os.LookupEnv(key); ok {
		return splitAndTrim(val, ",")
	}
	return splitAndTrim(defaultVal, ",")
}

func splitAndTrim(s string, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ============================
// Logger con Rotación de Logs
// ============================

// Logger define una estructura para manejar logs con diferentes niveles
type Logger struct {
	*log.Logger
}

// LoggerConfig configura el logger
type LoggerConfig struct {
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
	UseFile    bool
}

// NewLogger crea una nueva instancia de Logger
func NewLogger(config LoggerConfig) *Logger {
	var output io.Writer = os.Stdout
	if config.UseFile {
		output = &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
		}
	}

	return &Logger{
		Logger: log.New(output, "", log.LstdFlags|log.Lshortfile),
	}
}

// Métodos para diferentes niveles de log
func (l *Logger) Info(message string) {
	l.Println("[INFO] " + message)
}

func (l *Logger) Warn(message string) {
	l.Println("[WARN] " + message)
}

func (l *Logger) Error(message string) {
	l.Println("[ERROR] " + message)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.Printf("[INFO] "+format, v...)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Printf("[WARN] "+format, v...)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format, v...)
}

// ============================
// Interfaces y Modelos
// ============================

// PrinterManager interface para gestionar impresoras
type PrinterManager interface {
	ListPrinters() ([]string, error)
	PrinterExists(name string) (bool, error)
}

// DocumentPrinter interface para imprimir documentos
type DocumentPrinter interface {
	PrintFile(filePath, printer string) error
}

// DrawerOpener interface para abrir el cajón de la impresora
type DrawerOpener interface {
	OpenDrawer(printerName string) error
}

// PrinterService interface que combina todas las funcionalidades
type PrinterService interface {
	GetPrinters() ([]map[string]string, error)
	PrintPDFFromURL(fileURL, printerName string) error
	OpenDrawer(printerName string) error
}

// ============================
// Implementaciones Concretas
// ============================

// WindowsPrinterManager es una implementación de PrinterManager para Windows
type WindowsPrinterManager struct{}

// ListPrinters lista todas las impresoras instaladas en el sistema Windows incluyendo la ubicación
func (w WindowsPrinterManager) ListPrinters() ([]string, error) {
	cmd := exec.Command("powershell", "-Command",
		"Get-Printer | Select-Object Name, DriverName, PortName, PrinterStatus, Location | ForEach-Object { \"Name=$($_.Name);DriverName=$($_.DriverName);PortName=$($_.PortName);PrinterStatus=$($_.PrinterStatus);Location=$($_.Location)\" }")

	// Configura SysProcAttr para ocultar la ventana de PowerShell
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out // Captura también los errores

	// Ejecuta el comando
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error ejecutando PowerShell: %w, salida: %s", err, out.String())
	}

	// Procesa la salida en líneas y elimina caracteres de control
	lines := strings.Split(out.String(), "\n")
	var printers []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			printers = append(printers, trimmed)
		}
	}

	return printers, nil
}

// PrinterExists verifica si una impresora específica existe
func (w WindowsPrinterManager) PrinterExists(name string) (bool, error) {
	printers, err := w.ListPrinters()
	if err != nil {
		return false, fmt.Errorf("error al listar impresoras: %w", err)
	}
	for _, p := range printers {
		if strings.Contains(p, "Name="+name+";") {
			return true, nil
		}
	}
	return false, nil
}

// ExternalDocumentPrinter es una implementación de DocumentPrinter que utiliza un ejecutable externo
type ExternalDocumentPrinter struct {
	PDFPrinterPath string
}

// PrintFile imprime un archivo PDF en la impresora especificada
func (e ExternalDocumentPrinter) PrintFile(filePath, printer string) error {
	fmt.Printf("Imprimiendo archivo %s en impresora %s\n", filePath, printer)
	// Crea un comando para ejecutar el ejecutable de impresión
	cmd := exec.Command(e.PDFPrinterPath, filePath, printer)

	// Configura SysProcAttr para ocultar la ventana de la aplicación externa
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	/* 	cmd.Stderr = &bytes.Buffer{}
	   	cmd.Stdout = &bytes.Buffer{}
	*/
	cmd.Stderr = os.Stderr // Captura y muestra errores de impresión
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error al ejecutar PDFPrinter: %v, salida: %s", err, cmd.Stderr)
	}
	return nil
}

// WindowsDrawerOpener es una implementación de DrawerOpener para Windows
type WindowsDrawerOpener struct {
	DrawerCommandPath string
}

// OpenDrawer abre el cajón de la impresora especificada
func (w WindowsDrawerOpener) OpenDrawer(printerName string) error {
	// Ejecutar el script de PowerShell contenido en DrawerCommandPath
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", w.DrawerCommandPath, "-Printer", printerName)

	// Configura SysProcAttr para ocultar la ventana de PowerShell
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error al ejecutar comando de apertura de cajón: %v, salida: %s", err, string(output))
	}
	return nil
}

// DefaultPrinterService es la implementación por defecto de PrinterService
type DefaultPrinterService struct {
	PrinterManager  PrinterManager
	DocumentPrinter DocumentPrinter
	DrawerOpener    DrawerOpener
	Logger          *Logger
}

// GetPrinters obtiene la lista de impresoras con detalles
func (d DefaultPrinterService) GetPrinters() ([]map[string]string, error) {
	printerStrings, err := d.PrinterManager.ListPrinters()
	if err != nil {
		return nil, fmt.Errorf("error al listar impresoras: %w", err)
	}

	var printers []map[string]string
	for _, ps := range printerStrings {
		details, err := parsePrinterDetails(ps)
		if err != nil {
			d.Logger.Errorf("Error al parsear detalles de impresora: %v", err)
			continue
		}
		printers = append(printers, details)
	}

	return printers, nil
}

// PrintPDFFromURL descarga un PDF desde una URL y lo envía a la impresora especificada
func (d DefaultPrinterService) PrintPDFFromURL(fileURL, printerName string) error {
	exists, err := d.PrinterManager.PrinterExists(printerName)
	if err != nil {
		return fmt.Errorf("error al verificar la impresora: %w", err)
	}
	if !exists {
		return fmt.Errorf("la impresora '%s' no existe", printerName)
	}

	parsedURL, err := url.ParseRequestURI(fileURL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("esquema de URL no soportado: %s", parsedURL.Scheme)
	}

	filePath, err := downloadFile(fileURL)
	if err != nil {
		return fmt.Errorf("error al descargar el archivo: %w", err)
	}
	defer func() {
		if err := os.Remove(filePath); err != nil {
			d.Logger.Errorf("Error al eliminar archivo temporal: %v", err)
		}
	}()
	d.Logger.Infof("Archivo descargado: %s", filePath)
	if err := d.DocumentPrinter.PrintFile(filePath, printerName); err != nil {
		return fmt.Errorf("error al imprimir el archivo: %w", err)
	}
	return nil
}

// OpenDrawer abre el cajón de la impresora especificada
func (d DefaultPrinterService) OpenDrawer(printerName string) error {
	exists, err := d.PrinterManager.PrinterExists(printerName)
	if err != nil {
		return fmt.Errorf("error al verificar la impresora: %w", err)
	}
	if !exists {
		return fmt.Errorf("la impresora '%s' no existe", printerName)
	}

	if err := d.DrawerOpener.OpenDrawer(printerName); err != nil {
		return fmt.Errorf("error al abrir el cajón: %w", err)
	}
	return nil
}

// downloadFile descarga un archivo desde una URL y lo guarda temporalmente
func downloadFile(fileURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el servidor retornó estado no OK: %d %s", resp.StatusCode, resp.Status)
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

// parsePrinterDetails analiza una cadena de detalles de impresora y la convierte en un mapa
func parsePrinterDetails(details string) (map[string]string, error) {
	printerMap := make(map[string]string)
	properties := strings.Split(details, ";")
	for _, prop := range properties {
		kv := strings.SplitN(prop, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("formato de propiedad inválido: %s", prop)
		}
		printerMap[kv[0]] = kv[1]
	}
	return printerMap, nil
}

// ============================
// Handlers HTTP
// ============================

// Handlers agrupa todos los manejadores necesarios
type Handlers struct {
	Service PrinterService
	Logger  *Logger
}

// ListPrintersHandler maneja la solicitud para listar impresoras
func (h Handlers) ListPrintersHandler(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Received request: /list-printers")
	printers, err := h.Service.GetPrinters()
	if err != nil {
		h.Logger.Errorf("Error al listar impresoras: %v", err)
		WriteErrorJSON(w, http.StatusInternalServerError, "Error al listar las impresoras", err)
		return
	}

	response := map[string]interface{}{
		"printers": printers,
	}
	WriteJSON(w, http.StatusOK, response)
}

// PrintHandler maneja la solicitud para imprimir un PDF desde una URL
func (h Handlers) PrintHandler(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Received request: /print")

	if r.Method != http.MethodPost {
		h.Logger.Warnf("Método HTTP no permitido: %s", r.Method)
		WriteErrorJSON(w, http.StatusMethodNotAllowed, "Método HTTP no permitido", nil)
		return
	}

	// Obtener parámetros desde el cuerpo de la solicitud (mejor práctica que desde query params)
	type PrintRequest struct {
		URL     string `json:"url"`
		Printer string `json:"printer"`
	}

	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Logger.Warnf("Error al decodificar JSON: %v", err)
		WriteErrorJSON(w, http.StatusBadRequest, "Solicitud JSON inválida", err)
		return
	}

	if req.URL == "" || req.Printer == "" {
		h.Logger.Warn("URL o impresora no especificados")
		WriteErrorJSON(w, http.StatusBadRequest, "URL o impresora no especificados", nil)
		return
	}

	if err := h.Service.PrintPDFFromURL(req.URL, req.Printer); err != nil {
		h.Logger.Errorf("Error al imprimir: %v", err)
		WriteErrorJSON(w, http.StatusInternalServerError, "Error al imprimir el archivo", err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "PDF enviado a la impresora exitosamente."})
}

// OpenDrawerHandler maneja la solicitud para abrir el cajón de una impresora
func (h Handlers) OpenDrawerHandler(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Received request: /open-box")

	if r.Method != http.MethodPost {
		h.Logger.Warnf("Método HTTP no permitido: %s", r.Method)
		WriteErrorJSON(w, http.StatusMethodNotAllowed, "Método HTTP no permitido", nil)
		return
	}

	// Obtener parámetros desde el cuerpo de la solicitud
	type OpenDrawerRequest struct {
		Printer string `json:"printer"`
	}

	var req OpenDrawerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Logger.Warnf("Error al decodificar JSON: %v", err)
		WriteErrorJSON(w, http.StatusBadRequest, "Solicitud JSON inválida", err)
		return
	}

	if req.Printer == "" {
		h.Logger.Warn("No se especificó la impresora")
		WriteErrorJSON(w, http.StatusBadRequest, "No se especificó la impresora", nil)
		return
	}

	if err := h.Service.OpenDrawer(req.Printer); err != nil {
		h.Logger.Errorf("Error al abrir el cajón: %v", err)
		WriteErrorJSON(w, http.StatusInternalServerError, "Error al abrir el cajón", err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "Cajón abierto exitosamente."})
}

// HealthHandler maneja la solicitud de salud del servidor
func (h Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Received request: /health")
	WriteJSON(w, http.StatusOK, map[string]bool{"running": true})
}

// ============================
// Funciones Utilitarias
// ============================

// WriteJSON escribe una respuesta JSON con el estado especificado
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error al codificar respuesta JSON: %v", err)
	}
}

// WriteErrorJSON escribe una respuesta de error en formato JSON
func WriteErrorJSON(w http.ResponseWriter, status int, message string, err error) {
	resp := map[string]string{"error": message}
	if err != nil {
		resp["details"] = err.Error()
	}
	WriteJSON(w, status, resp)
}

// ============================
// Función Principal
// ============================

func main() {
	// Cargar configuración
	cfg := LoadConfig()

	// Configurar logger
	loggerConfig := LoggerConfig{
		Filename:   cfg.LogFile,
		MaxSize:    cfg.LogMaxSize,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAge,
		Compress:   cfg.LogCompress,
		UseFile:    true,
	}
	logger := NewLogger(loggerConfig)

	// Inicializar servicios
	pm := WindowsPrinterManager{}
	dp := ExternalDocumentPrinter{PDFPrinterPath: cfg.PDFPrinterPath}
	do := WindowsDrawerOpener{DrawerCommandPath: cfg.DrawerCommandPath}

	service := DefaultPrinterService{
		PrinterManager:  pm,
		DocumentPrinter: dp,
		DrawerOpener:    do,
		Logger:          logger,
	}

	// Inicializar manejadores
	handlers := Handlers{
		Service: service,
		Logger:  logger,
	}

	// Configurar rutas
	mux := http.NewServeMux()
	mux.HandleFunc("/print", handlers.PrintHandler)
	mux.HandleFunc("/open-box", handlers.OpenDrawerHandler)
	mux.HandleFunc("/list-printers", handlers.ListPrintersHandler)
	mux.HandleFunc("/health", handlers.HealthHandler)

	// Configurar CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "Accept", "authorization", "x-app-version"},
		AllowCredentials: false,
		MaxAge:           300, // 5 minutos
		Debug:            false,
	})

	handlerWithCORS := c.Handler(mux)

	// Configurar servidor HTTP
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handlerWithCORS,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.HTTPIdleTimeout) * time.Second,
	}

	logger.Infof("Servidor iniciado en puerto :%d", cfg.Port)

	// Iniciar servidor con o sin TLS
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		logger.Infof("Iniciando servidor TLS")
		log.Fatal(server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}
