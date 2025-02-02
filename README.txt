# PrinterMatiasERP

## Descripción

PrinterMatiasERP es un servidor HTTP que permite:
- Listar impresoras instaladas en el sistema.
- Imprimir archivos PDF enviándoles una URL remota.
- Abrir el cajón de efectivo de la impresora.

Este servidor está pensado para integrarse con un sistema ERP o una SPA (Single Page Application) que requiera funcionalidad de impresión local.

## Requisitos del Sistema

- **Sistema Operativo:** Windows 10 o superior.
- **Permisos de impresión:** El usuario que ejecuta el servidor debe tener permisos para usar las impresoras.
- **Red/Firewall:** El servidor por defecto escucha en el puerto 8080. Si deseas acceder desde otras máquinas, verifica que el firewall no bloquee el puerto.

## Archivos Incluidos

- `PrinterMatiasERP.exe`: Ejecutable principal del servidor.
- `PDFtoPrinter.exe`: Herramienta externa utilizada para enviar PDFs a la impresora.
- `drawer_open_command.txt`: Archivo de comando que contiene la secuencia para abrir el cajón.
- `README.txt`: Este documento con las instrucciones.
- `.env`: Archivo opcional para configurar variables de entorno.

## Variables de Entorno (Opcional)

En el archivo `.env` puedes definir las siguientes variables:

- `PORT`: Puerto en el que se inicia el servidor (por defecto, 8080).
- `PDF_PRINTER_PATH`: Ruta hacia el ejecutable `PDFtoPrinter.exe` (por defecto, `./PDFtoPrinter.exe`).
- `DRAWER_COMMAND_PATH`: Ruta hacia el archivo de comando del cajón (por defecto, `./drawer_open_command.txt`).

Si no utilizas `.env`, el servidor tomará los valores por defecto.

## Uso

1. **Instalación**:  
   Después de ejecutar el instalador, se copiarán los archivos al directorio de instalación.

2. **Configuración (opcional)**:  
   - Edita el archivo `.env` para cambiar el puerto u otras variables si es necesario.
   - Asegúrate de que `PDFtoPrinter.exe` y `drawer_open_command.txt` estén en el mismo directorio que `PrinterMatiasERP.exe`.

3. **Ejecución**:  
   - Abre una terminal en el directorio de instalación.
   - Ejecuta:
     ```bash
     PrinterMatiasERP.exe
     ```
   - El servidor iniciará en el puerto definido en `.env` o por defecto en `http://localhost:8080`.

## Endpoints Disponibles

- **Health Check**: `GET /health`  
  Retorna `{"running": true}` si el servidor está operativo.

- **Listar Impresoras**: `GET /list-printers`  
  Devuelve un arreglo JSON con las impresoras instaladas.

- **Imprimir PDF**: `GET /print?url=<URL_PDF>&printer=<NOMBRE_IMPRESORA>`  
  Descarga el PDF desde la URL especificada y lo envía a la impresora indicada.  
  Ejemplo: `http://localhost:8080/print?url=http://example.com/documento.pdf&printer=MiImpresora`

- **Abrir Cajón**: `GET /open-box?printer=<NOMBRE_IMPRESORA>`  
  Envía el comando para abrir el cajón de la impresora.  
  Ejemplo: `http://localhost:8080/open-box?printer=MiImpresora`

## Solución de Problemas

- **No se puede imprimir**:  
  Asegúrate de que `PDFtoPrinter.exe` esté presente y que el nombre de la impresora sea correcto.

- **No abre el cajón**:  
  Verifica que `drawer_open_command.txt` contenga la secuencia correcta para tu impresora.

- **No accede al servidor**:  
  Revisa el firewall, antivirus o utiliza `http://localhost:8080/health` para confirmar que el servidor está en ejecución.

## Contacto y Soporte

Para consultas o reportes de problemas, contacta al equipo de soporte técnico de MatiasERP.
Corre: soporte@matias.com.co