# Iconos de devManager

Esta carpeta contiene los iconos de la aplicación extraídos de [Reicon](https://reicon.dev/).

## Iconos disponibles

- `logo.svg` - Logo principal de la aplicación
- `server.svg` - Servidores y gestión de puertos
- `play.svg` - Playwright y tests automatizados
- `code.svg` - Scripts y código
- `git-branch.svg` - Control de versiones Git
- `monitor.svg` - Monitor de recursos y procesos
- `bug.svg` - Reporte de errores y evidencias
- `folder.svg` - Proyectos y directorios
- `settings.svg` - Configuración

## Cómo usar los iconos

### Opción 1: Importación dinámica (recomendado)
```javascript
import { serverIcon, playIcon } from './icons/index.js';

// Usar como componente
const icon = await serverIcon();
```

### Opción 2: Importación directa
```javascript
import { serverSvg, playSvg } from './icons/index.js';

// Usar directamente en HTML
element.innerHTML = serverSvg;
```

### Opción 3: En HTML
```html
<!-- Como imagen -->
<img src="./src/icons/server.svg" alt="Server">

<!-- Como SVG inline (requiere configuración del bundler) -->
<svg><!-- ... --></svg>
```

## Estilo de los iconos

Todos los iconos usan:
- `stroke="currentColor"` - Se adaptan al color del texto
- `stroke-width="1.5"` - Grosor consistente
- `width="24" height="24"` - Tamaño estándar (excepto logo que es 32x32)
- `fill="none"` - Solo contorno (estilo outline)

## Créditos

Iconos extraídos de [Reicon](https://reicon.dev/) - Biblioteca de iconos open source.