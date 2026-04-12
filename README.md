# pipnest

`pipnest` es una TUI (terminal UI) para trabajar con dependencias de Python y entornos virtuales desde una única interfaz interactiva.

## Características

- Búsqueda de paquetes en PyPI con ranking difuso.
- Gestión de paquetes instalados (listar, instalar, desinstalar, freeze).
- Gestión de intérpretes y entornos virtuales.
- Cheatsheet integrado para comandos comunes de Python/pip.

## Instalación

### Como dependencia del módulo (go get)

```bash
go get github.com/Rotlerxd/pipnest@latest
```

### Como binario CLI

```bash
go install github.com/Rotlerxd/pipnest@latest
```

## Uso

```bash
pipnest
```

También puedes abrir una pantalla concreta:

```bash
pipnest -screen search
pipnest -screen venvs
```

## Ejemplo rápido

1. Ejecuta `pipnest`.
2. Entra a **Packages**.
3. Escribe un nombre de paquete (ej. `requests`) y presiona `Enter`.
4. Navega los resultados y detalles con teclado/foco de panel.

## Casos de uso

- Equipos que mantienen varios entornos virtuales por proyecto.
- Flujos rápidos de revisión de dependencias sin salir de terminal.
- Soporte a onboarding de Python con referencias de comandos integradas.

## Desarrollo

```bash
go fmt ./...
go vet ./...
go test ./...
```

## Plataformas soportadas

- Linux
- macOS

Windows no está soportado por diseño (build tags del proyecto).

## Licencia

MIT. Ver [`LICENSE`](./LICENSE).
