# Project Rules — devManager

## build.bat popup blocks (CRITICAL)

`build.bat` genera los popups (success/error) escribiendo un `.ps1` temporal desde un bloque cmd:

```
> "!PS_SCRIPT!" (
    echo ...
)
```

Reglas de oro al editar cualquier línea `echo` DENTRO de esos bloques:

1. **Todo `)` debe escaparse como `^)`.** En bloques `( )`, un `)` sin escapar CIERRA el bloque prematuramente. El `(` NO necesita escape. Las comillas dobles protegen solo dentro de la línea; una comilla abierta se cierra a fin de línea.
2. **`|`, `&`, `<`, `>` dentro del bloque también deben escaparse** (`^|`, `^&`, etc.) si el texto a escribir los contiene.
3. **Los `^` se consumen al generar el .ps1** — el archivo resultante contiene el carácter literal (ej. `DllImport("user32.dll"^)` escribe `DllImport("user32.dll")` en el ps1).
4. Si el bloque se rompe, cmd falla con `] was unexpected at this time` o similar, el `.ps1` nunca se genera y el popup no aparece. Síntoma clásico: popup que "desaparece" tras un edit inocente.

Síntoma previo en historial: la línea `SetForegroundWindow(IntPtr hWnd)` rompió el popup de éxito por falta de `^)`.

Verificar después de tocar el bloque: generar el bloque y parsear el ps1 resultante con el parser de PowerShell (o ejecutar el bloque aislado en un wrapper bat).