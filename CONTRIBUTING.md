# Guia rapida para devs

Este proyecto esta en una fase inicial. La idea es crecer por modulos y mantener cada pieza simple de entender.

## Estructura actual

- `main.go`: entrypoint de la app.
- `model.go`: estado principal y ciclo Update/View de Bubble Tea.
- `menu.go`: base de navegacion para menu principal y submenus.
- `internal/pkgsearch`: busqueda de paquetes, lógica de PyPI y UI de resultados.
- `internal/requirements`: manejo de requirements y futura UI del flujo.
- `internal/venvs`: manejo de entornos virtuales y futura UI del flujo.

## Idea de equipos

- Equipo de paquetes: trabaja dentro de `internal/pkgsearch`.
- Equipo de requirements: trabaja dentro de `internal/requirements`.
- Equipo de venvs: trabaja dentro de `internal/venvs`.
- Equipo de shell: coordina `menu.go` y `model.go`.

## Como trabajar en este repo

1. Hacer cambios pequenos y enfocados.
2. Separar responsabilidades por carpeta o archivo segun dominio.
3. Evitar meter logica nueva en `main.go`.
4. Validar que no haya errores de compilacion antes de cerrar cambios.

## Proximo crecimiento sugerido

1. Implementar navegacion real usando `ScreenID` en `menu.go`.
2. Crear vistas por seccion:
   - requirements
   - packages
   - venvs
3. Mantener cada seccion con su propio estado y funciones de render.

## Criterios de calidad basicos

- Nombres claros en tipos y funciones.
- Funciones cortas cuando sea posible.
- Sin side-effects ocultos en render.
- Errores siempre manejados y mostrados al usuario.
