' Ensure UTF-8 encoding for Spanish characters
Set WshShell = CreateObject("WScript.Shell")
Set FSO = CreateObject("Scripting.FileSystemObject")

' Function to display messages with proper UTF-8 encoding
Function ShowMessage(title, message, buttons, icon)
    ShowMessage = WshShell.Popup(message, 0, title, buttons + icon)
End Function

' Set working directory to the project folder
ScriptDir = FSO.GetParentFolderName(WScript.ScriptFullName)
WshShell.CurrentDirectory = ScriptDir

' Check if devManager is already running
Dim isRunning, pythonwProcess
isRunning = False
On Error Resume Next
For Each process in GetObject("winmgmts:").ExecQuery("Select * from Win32_Process Where Name='pythonw.exe'")
    If InStr(process.CommandLine, "main.py") > 0 Then
        isRunning = True
        pythonwProcess = process.ProcessId
        Exit For
    End If
Next
On Error GoTo 0

If isRunning Then
    result = MsgBox("devManager ya est" & ChrW(225) & " en ejecuci" & ChrW(243) & "n." & vbCrLf & vbCrLf & _
                   ChrW(191) & "Qu" & ChrW(233) & " desea hacer?" & vbCrLf & vbCrLf & _
                   "S" & ChrW(237) & " - Cerrar la instancia actual y abrir una nueva" & vbCrLf & _
                   "No - Continuar con la instancia ya abierta", _
                   vbQuestion + vbYesNo, "devManager - Ya en ejecuci" & ChrW(243) & "n")
    
    If result = vbYes Then
        ' Terminate existing process
        On Error Resume Next
        WshShell.Run "taskkill /F /PID " & pythonwProcess, 0, True
        On Error GoTo 0
        ' Wait a moment for process to terminate
        WScript.Sleep 1000
    Else
        ' Exit without opening new instance
        WScript.Quit 0
    End If
End If

' Check if this is the first run
Dim firstRunFile
firstRunFile = ScriptDir & "\.firstrun"

If Not FSO.FileExists(firstRunFile) Then
    ' First time running the application
    result = MsgBox(ChrW(161) & "Bienvenido a devManager!" & vbCrLf & vbCrLf & _
                    "Parece que es tu primera vez. " & ChrW(191) & "Desea configurar todo autom" & ChrW(225) & "ticamente?" & vbCrLf & vbCrLf & _
                    "Esto crear" & ChrW(225) & " el entorno virtual e instalar" & ChrW(225) & " las dependencias necesarias." & vbCrLf & _
                    "El proceso puede tomar unos minutos.", _
                    vbInformation + vbYesNo, "devManager - Configuraci" & ChrW(243) & "n Inicial")
    
    If result = vbYes Then
        ' Show progress and run setup with retry logic
        Dim retryCount, setupSuccess, progressMessage
        retryCount = 0
        setupSuccess = False
        
        Do While retryCount < 3 And Not setupSuccess
            progressMessage = "Configurando devManager por primera vez..."
            If retryCount > 0 Then
                progressMessage = progressMessage & " (Intento " & (retryCount + 1) & " de 3)"
            End If
            ShowProgressPopup progressMessage
            
            ' Run setup and capture exit code
            WshShell.Run "powershell -ExecutionPolicy Bypass -File """ & ScriptDir & "\setup.ps1"" -Mode setup", 1, True
            
            ' Check if setup was successful
            If FSO.FolderExists(ScriptDir & "\.venv") And FSO.FileExists(ScriptDir & "\.venv\Scripts\pythonw.exe") Then
                setupSuccess = True
            Else
                retryCount = retryCount + 1
                If retryCount < 3 Then
                    result = MsgBox("La configuraci" & ChrW(243) & "n no pudo completarse." & vbCrLf & vbCrLf & _
                                   ChrW(191) & "Desea intentar nuevamente? (Intento " & (retryCount + 1) & " de 3)", _
                                   vbExclamation + vbYesNo, "devManager - Error en Configuraci" & ChrW(243) & "n")
                    If result = vbNo Then
                        Exit Do
                    End If
                End If
            End If
        Loop
        
        If setupSuccess Then
            FSO.CreateTextFile(firstRunFile, True).Close ' Mark as configured
            MsgBox ChrW(161) & "Configuraci" & ChrW(243) & "n completada exitosamente!" & vbCrLf & vbCrLf & _
                   "devManager est" & ChrW(225) & " listo para usar.", _
                   vbInformation, "devManager - Listo"
            ' Now try to launch the app
            WshShell.Run """" & ScriptDir & "\.venv\Scripts\pythonw.exe"" """ & ScriptDir & "\main.py""", 0, False
            WScript.Quit 0
        Else
            MsgBox "La configuraci" & ChrW(243) & "n no pudo completarse despu" & ChrW(233) & "s de 3 intentos." & vbCrLf & vbCrLf & _
                   "Por favor, ejecute setup.ps1 manualmente para solucionar el problema." & vbCrLf & _
                   "Posibles causas:" & vbCrLf & _
                   "- Python no est" & ChrW(225) & " instalado correctamente" & vbCrLf & _
                   "- Permisos insuficientes en esta carpeta" & vbCrLf & _
                   "- Antivirus bloqueando la ejecuci" & ChrW(243) & "n", _
                   vbExclamation, "devManager - Error en Configuraci" & ChrW(243) & "n"
            WScript.Quit 1
        End If
    Else
        MsgBox "Puede configurar devManager m" & ChrW(225) & "s tarde ejecutando setup.ps1 manualmente." & vbCrLf & vbCrLf & _
               "La aplicaci" & ChrW(243) & "n no puede iniciarse sin configuraci" & ChrW(243) & "n previa.", _
               vbInformation, "devManager - Informaci" & ChrW(243) & "n"
        WScript.Quit 0
    End If
End If

' Verify critical files exist before proceeding
If Not FSO.FileExists(ScriptDir & "\main.py") Then
    MsgBox "Error: No se encuentra el archivo principal de la aplicaci" & ChrW(243) & "n." & vbCrLf & vbCrLf & _
           "Aseg" & ChrW(250) & "rese de que main.py exista en la carpeta principal.", _
           vbCritical, "devManager - Archivo Faltante"
    WScript.Quit 1
End If

' Check available disk space (minimum 500MB)
On Error Resume Next
Dim driveObj
Set driveObj = FSO.GetDrive(ScriptDir)
If Err.Number = 0 Then
    If driveObj.AvailableSpace < 500 * 1024 * 1024 Then
        result = MsgBox("Advertencia: Hay menos de 500MB de espacio disponible." & vbCrLf & vbCrLf & _
                        "La instalaci" & ChrW(243) & "n podr" & ChrW(237) & "a fallar. " & ChrW(191) & "Desea continuar?", _
                        vbExclamation + vbYesNo, "devManager - Espacio Insuficiente")
        If result = vbNo Then
            WScript.Quit 1
        End If
    End If
End If
On Error GoTo 0

' Security validation: Check if setup.ps1 has been tampered with
If FSO.FileExists(ScriptDir & "\setup.ps1") Then
    Dim psFile, psContent
    Set psFile = FSO.OpenTextFile(ScriptDir & "\setup.ps1", 1)
    psContent = psFile.ReadAll
    psFile.Close
    
    ' Basic security checks
    Dim securityIssues
    securityIssues = ""
    
    ' Check for suspicious commands
    If InStr(LCase(psContent), "format") > 0 And InStr(LCase(psContent), "c:") > 0 Then
        securityIssues = securityIssues & "- Comandos de formato de disco detectados" & vbCrLf
    End If
    
    If InStr(LCase(psContent), "del ") > 0 And InStr(LCase(psContent), "c:\windows") > 0 Then
        securityIssues = securityIssues & "- Comandos de eliminaci" & ChrW(243) & "n en sistema detectados" & vbCrLf
    End If
    
    If InStr(LCase(psContent), "rundll32") > 0 And InStr(LCase(psContent), "user32") > 0 Then
        securityIssues = securityIssues & "- Llamadas sospechosas a DLL del sistema" & vbCrLf
    End If
    
    ' Check for essential devManager components
    If InStr(psContent, "main.py") = 0 Then
        securityIssues = securityIssues & "- Falta componente esencial de devManager" & vbCrLf
    End If
    
    If InStr(psContent, "devManager") = 0 Then
        securityIssues = securityIssues & "- Referencias a devManager modificadas" & vbCrLf
    End If
    
    If securityIssues <> "" Then
        result = MsgBox("Advertencia de seguridad:" & vbCrLf & vbCrLf & _
                       "Se detectaron posibles problemas en setup.ps1:" & vbCrLf & _
                       securityIssues & vbCrLf & _
                       ChrW(191) & "Desea continuar ejecutando el archivo?", _
                       vbExclamation + vbYesNo, "devManager - Advertencia de Seguridad")
        If result = vbNo Then
            MsgBox "Ejecuci" & ChrW(243) & "n cancelada por razones de seguridad." & vbCrLf & vbCrLf & _
                   "Por favor, revise el archivo setup.ps1 o descargue una versi" & ChrW(243) & "n limpia.", _
                   vbCritical, "devManager - Seguridad"
            WScript.Quit 1
        End If
    End If
Else
    MsgBox "Error: No se encuentra el archivo setup.ps1." & vbCrLf & vbCrLf & _
           "Este archivo es necesario para la configuraci" & ChrW(243) & "n inicial.", _
           vbCritical, "devManager - Archivo Faltante"
    WScript.Quit 1
End If

Sub ShowProgressPopup(message)
    ' Show non-blocking popup with information
    WshShell.Popup message & vbCrLf & vbCrLf & _
                   "Por favor, espere. Esto puede tomar varios minutos...", 3, "devManager - Configurando", vbInformation
End Sub

' Check if virtual environment exists
If Not FSO.FolderExists(ScriptDir & "\.venv") Then
    result = MsgBox("El entorno virtual no existe." & vbCrLf & vbCrLf & _
                    ChrW(191) & "Desea ejecutar setup.ps1 para crearlo autom" & ChrW(225) & "ticamente?", _
                    vbExclamation + vbYesNo, "devManager - Configuraci" & ChrW(243) & "n Requerida")
    
    If result = vbYes Then
        ' Execute setup.ps1
        WshShell.Run """" & ScriptDir & "\setup.ps1""", 1, True
        WScript.Quit 0
    Else
        MsgBox "La aplicaci" & ChrW(243) & "n no puede iniciarse sin el entorno virtual." & vbCrLf & vbCrLf & _
               "Ejecute setup.ps1 manualmente cuando est" & ChrW(233) & " listo.", _
               vbInformation, "devManager - Informaci" & ChrW(243) & "n"
        WScript.Quit 1
    End If
End If

' Check if pythonw exists in virtual environment
If Not FSO.FileExists(ScriptDir & "\.venv\Scripts\pythonw.exe") Then
    result = MsgBox("Python no encontrado en el entorno virtual." & vbCrLf & vbCrLf & _
                    "El entorno puede estar da" & ChrW(241) & "ado. " & ChrW(191) & "Desea repararlo?", _
                    vbExclamation + vbYesNo, "devManager - Entorno Virtual Da" & ChrW(241) & "ado")
    
    If result = vbYes Then
        ' Delete corrupted venv and run setup
        On Error Resume Next
        FSO.DeleteFolder ScriptDir & "\.venv", True
        WshShell.Run """" & ScriptDir & "\setup.ps1""", 1, True
        WScript.Quit 0
    Else
        MsgBox "La aplicaci" & ChrW(243) & "n no puede iniciarse sin Python en el entorno virtual." & vbCrLf & vbCrLf & _
               "Elimine manualmente la carpeta .venv y ejecute setup.ps1.", _
               vbInformation, "devManager - Informaci" & ChrW(243) & "n"
        WScript.Quit 1
    End If
End If

' Check if main.py exists
If Not FSO.FileExists(ScriptDir & "\main.py") Then
    MsgBox "Error: Archivo main.py no encontrado." & vbCrLf & vbCrLf & _
           "Aseg" & ChrW(250) & "rese de que el archivo principal de la aplicaci" & ChrW(243) & "n existe.", _
           vbCritical, "devManager - Archivo Faltante"
    WScript.Quit 1
End If

' Try to launch pythonw without creating a console window
On Error Resume Next
WshShell.Run """" & ScriptDir & "\.venv\Scripts\pythonw.exe"" """ & ScriptDir & "\main.py""", 0, False

If Err.Number <> 0 Then
    result = MsgBox("Error al iniciar la aplicaci" & ChrW(243) & "n:" & vbCrLf & vbCrLf & _
                    "Descripci" & ChrW(243) & "n: " & Err.Description & vbCrLf & _
                    "C" & ChrW(243) & "digo: " & Err.Number & vbCrLf & vbCrLf & _
                    ChrW(191) & "Desea intentar reparar el problema?", _
                    vbExclamation + vbYesNo, "devManager - Error de Inicio")
    
    If result = vbYes Then
        ' Try to fix by running setup
        WshShell.Run """" & ScriptDir & "\setup.ps1""", 1, True
        WScript.Quit 0
    Else
        MsgBox "No se pudo iniciar la aplicaci" & ChrW(243) & "n." & vbCrLf & vbCrLf & _
               "Soluciones manuales:" & vbCrLf & _
               "1. Ejecute setup.ps1 para reinstalar dependencias" & vbCrLf & _
               "2. Verifique que Python est" & ChrW(233) & " instalado correctamente" & vbCrLf & _
               "3. Reinicie su sistema si el problema persiste", _
               vbInformation, "devManager - Informaci" & ChrW(243) & "n"
        WScript.Quit 1
    End If
End If

On Error GoTo 0
