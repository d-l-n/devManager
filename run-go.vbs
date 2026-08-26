' devManager (Go/Wails) launcher
' Launches the self-contained Go binary. No Python/venv/setup required.
Set WshShell = CreateObject("WScript.Shell")
Set FSO = CreateObject("Scripting.FileSystemObject")

' Set working directory to the repo root so projects.json is shared with the Python app
ScriptDir = FSO.GetParentFolderName(WScript.ScriptFullName)
WshShell.CurrentDirectory = ScriptDir

ExePath = ScriptDir & "\desktop-go\build\bin\devManager.exe"

If Not FSO.FileExists(ExePath) Then
    MsgBox "No se encuentra el binario de devManager (Go):" & vbCrLf & vbCrLf & _
           "desktop-go\build\bin\devManager.exe" & vbCrLf & vbCrLf & _
           "Compilalo con:  wails build  (dentro de desktop-go)", _
           vbCritical, "devManager - Binario no encontrado"
    WScript.Quit 1
End If

' Check if already running (single instance)
On Error Resume Next
For Each process In GetObject("winmgmts:").ExecQuery("Select * from Win32_Process Where Name='devManager.exe'")
    result = MsgBox("devManager ya est" & ChrW(225) & " en ejecuci" & ChrW(243) & "n." & vbCrLf & vbCrLf & _
                    ChrW(191) & "Abrir otra instancia de todos modos?", _
                    vbQuestion + vbYesNo, "devManager")
    If result <> vbYes Then WScript.Quit 0
    Exit For
Next
On Error GoTo 0

WshShell.Run """" & ExePath & """", 0, False
