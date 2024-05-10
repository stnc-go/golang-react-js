PowerShell says "execution of scripts is disabled on this system."

For Windows 11, Windows 10, Windows 7, Windows 8, Windows Server 2008 R2 or Windows Server 2012, run the following commands as Administrator:

x86 (32 bit)
Open C:\Windows\SysWOW64\cmd.exe
Run the command powershell Set-ExecutionPolicy RemoteSigned

x64 (64 bit)
Open C:\Windows\system32\cmd.exe
Run the command powershell Set-ExecutionPolicy RemoteSigned

You can check mode using

    In CMD: echo %PROCESSOR_ARCHITECTURE%
    In Powershell: [Environment]::Is64BitProcess