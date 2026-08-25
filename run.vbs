Set WshShell = CreateObject("WScript.Shell")
Set FSO = CreateObject("Scripting.FileSystemObject")

' Set working directory to the project folder
ScriptDir = FSO.GetParentFolderName(WScript.ScriptFullName)
WshShell.CurrentDirectory = ScriptDir

' Launch pythonw without creating a console window
WshShell.Run """" & ScriptDir & "\.venv\Scripts\pythonw.exe"" """ & ScriptDir & "\main.py""", 0, False
