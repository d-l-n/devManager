# Backlog de devManager

*Este documento es local y no debe subirse al repositorio*

## 📊 Resumen
- **Total tareas**: 10
- **Completadas**: 2 (20%)
- **En progreso**: 1 (10%)
- **Pendientes**: 7 (70%)

---

## ✅ Completadas

### 1. Modo OLED en ajustes
- **Estado**: ✅ Completado
- **Descripción**: Ya estaba implementado y funcional
- **Acceso**: Settings → Appearance → OLED (radio button)
- **Características**: Fondo negro puro (#000000) para ahorro en pantallas OLED

### 2. Tema automático del sistema
- **Estado**: ✅ Completado
- **Descripción**: Detecta y aplica tema según configuración del SO
- **Implementación**:
  - Windows: Registro del sistema via PowerShell
  - macOS: `defaults read -g AppleInterfaceStyle`
  - Linux: `gsettings` o variable GTK_THEME
- **Nueva opción**: "Auto" en el ciclo de temas

---

## 🔄 En Progreso

### 3. Mejorar UX de búsqueda de proyectos
- **Estado**: 🔄 En progreso
- **Descripción**: Implementar búsqueda avanzada con filtros y selección mejorada
- **Ideas a implementar**:
  - Búsqueda en tiempo real
  - Filtros por tipo de proyecto
  - Ordenamiento personalizable
  - Atajos de teclado

---

## ⏳ Pendientes

### 4. Crear proyectos desde la app
- **Prioridad**: Alta
- **Descripción**: Añadir formulario para crear nuevos proyectos sin editar JSON manualmente
- **Campos necesarios**:
  - Nombre del proyecto
  - Ruta del proyecto
  - Tipo de proyecto (web, mobile, api, etc.)
  - Comandos personalizados

### 5. Mejorar robustez de setup inicial
- **Prioridad**: Alta
- **Descripción**: Manejo de dependencias faltantes o programas que impiden ejecución
- **Casos a cubrir**:
  - Node.js no instalado
  - Go no instalado
  - Playwright no configurado
  - Puertos en uso

### 6. Investigar sistema de temas
- **Prioridad**: Media
- **Descripción**: Análisis profundo de arquitectura de temas para posibles mejoras
- **Investigación**:
  - Temas personalizados
  - Exportar/importar temas
  - Temas por proyecto

### 7. Personalizar popups e interfaces
- **Prioridad**: Media
- **Descripción**: Mejorar diálogos y notificaciones
- **Mejoras**:
  - Animaciones suaves
  - Posicionamiento inteligente
  - Tema consistente

### 8. Implementar indicador de versión
- **Prioridad**: Baja
- **Descripción**: Mostrar versión actual de la aplicación
- **Ubicación**: About dialog o status bar

### 9. Reorganizar archivos sueltos en carpetas
- **Prioridad**: Media
- **Descripción**: Organizar estructura del proyecto
- **Archivos a organizar**:
  - Scripts de build
  - Archivos de configuración
  - Documentación

### 10. Crear cmd para build en Windows
- **Prioridad**: Media
- **Descripción**: Script para ejecutar build-final.ps1 fácilmente
- **Implementación**: Archivo .cmd o .bat que llame al script PowerShell

---

## 📝 Notas

### Decisiones Arquitectónicas
- El tema OLED ya estaba implementado, solo se verificó funcionalidad
- El tema auto requiere detección por SO para máxima compatibilidad
- Los settings se persisten en JSON local (no en projects.json)

### Próximos Pasos
1. Terminar UX de proyectos (tarea 3)
2. Priorizar creación de proyectos desde app (tarea 4)
3. Mejorar robustez del setup (tarea 5)

---

*Última actualización: 2026-08-26*