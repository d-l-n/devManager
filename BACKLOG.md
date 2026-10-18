# Project Backlog — devManager

Lista de tareas pendientes, mejoras planificadas y deuda técnica del proyecto.

---

## 📌 Pendientes (To Do)

### 🎨 Temas y Apariencia
- [ ] **Mejorar el modo OLED**
  - **Prioridad**: Media
  - **Descripción**: Optimizar contrastes, sombras, bordes y elementos de interfaz para paneles OLED con fondos negro puro (`#000000`), asegurando legibilidad sin comprometer el ahorro de energía.
  - **Puntos clave**:
    - Calibrar la relación de contraste de textos secundarios y terciarios (`--text-secondary`, `--fg-dim`) sobre fondo `#000000`.
    - Ajustar bordes sutiles (`--border-subtle`) en paneles y tarjetas para evitar bordes duros o invisibles.
    - Revisar ticks de checkboxes y radios en modo OLED con acentos claros.
    - Asegurar que los componentes de logs, terminales y tablas de monitoreo mantengan consistencia con negros absolutos.

---

## 🚀 En Progreso (In Progress)
*(Sin tareas activas actualmente)*

---

## ✅ Completadas (Done)
- [x] **Unificación de estilo visual**: Retención exclusiva del estilo `standard` con soporte para Dark, Light, OLED y System, eliminando estilos redundantes (`brutalist`, `glassmorphism`, `retro`, `dracula`).
- [x] **Compatibilidad 120Hz+**: Integración de flags de aceleración por hardware en WebView2, capas del compositor CSS y auto-scroll de logs mediante `requestAnimationFrame`.
- [x] **Corrección de navegación y superposiciones**: Garantizar que el cambio de vistas oculte adecuadamente secciones inactivas y añadir botón de retorno y tecla Escape.
- [x] **Estilos nativos personalizados**: Rediseño temático completo de sliders, selects, pickers de color y botones secundarios.
