package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"log/slog"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procFindWindow         = user32.NewProc("FindWindowW")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procFindWindowEx       = user32.NewProc("FindWindowExW")
	procSetParent          = user32.NewProc("SetParent")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	messageBox             = user32.NewProc("MessageBoxW")
)

const (
	SMTO_NORMAL  = 0x0000
	SM_CXSCREEN  = 0
	SM_CYSCREEN  = 1
	SWP_NOZORDER = 0x0004
	MyApp        = "ElyWE"
	Version      = "0.0.2"
	Author       = "aiko-chan-ai"
)

const (
	MB_ICONERROR       = 0x00000010
	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
)

func UTF16PtrFromString(s string) *uint16 {
	strPtr, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	return strPtr
}

func stopMPV() error {
	// Check if any mpv processes are running
	checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq mpv.exe")
	var out bytes.Buffer
	checkCmd.Stdout = &out

	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("failed to check for mpv processes: %v", err)
	}

	if !bytes.Contains(out.Bytes(), []byte("mpv.exe")) {
		return fmt.Errorf("no mpv processes found")
	}

	killCmd := exec.Command("taskkill", "/F", "/IM", "mpv.exe")
	if err := killCmd.Run(); err != nil {
		return fmt.Errorf("failed to kill mpv processes: %v\n", err)
	}

	slog.Debug("mpv processes killed successfully.")
	return nil
}

func checkPath(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path is not absolute")
	}
	file, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found")
	}
	if file.IsDir() {
		return fmt.Errorf("path is a directory")
	}
	return nil
}

type CheckKeyErrors int

const (
	KeyNotFound CheckKeyErrors = iota
	VideoPathNotFound
	FileInvalid
)

func checkKey() CheckKeyErrors {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.QUERY_VALUE)
	if err != nil {
		return KeyNotFound
	}
	defer key.Close()

	path, _, err := key.GetStringValue("VideoPath")
	if err != nil {
		return VideoPathNotFound
	}

	if err := checkPath(path); err != nil {
		return FileInvalid
	}

	return 0 // No error

}

func getExecPathAndDir() (string, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", "", err
	}
	return exePath, filepath.Dir(exePath), nil
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func runMeElevated() error {
	verb := "runas"
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	showCmd := int32(1) //SW_NORMAL
	if err := windows.ShellExecute(0, UTF16PtrFromString(verb), UTF16PtrFromString(exe), UTF16PtrFromString(strings.Join(os.Args[1:], " ")), UTF16PtrFromString(cwd), showCmd); err != nil {
		return err
	}
	return nil
}

// List of video file extensions
var videoExtensions = []string{
	".mp4",
	".avi",
	".mkv",
	".mov",
	".wmv",
	".flv",
	".webm",
}

func showMessageBox(title, text string, style uintptr) error {
	_, _, err := messageBox.Call(
		0,
		uintptr(unsafe.Pointer(UTF16PtrFromString(text))),
		uintptr(unsafe.Pointer(UTF16PtrFromString(MyApp+" - "+title))),
		style,
	)

	return err
}

func createContextMenu(ext string, exePath string) error {
	shellKeyPath := fmt.Sprintf(`SystemFileAssociations\%s\shell\SetAsDesktopBackground`, ext)
	shellKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, shellKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create shell registry key for %s: %v", ext, err)
	}
	defer shellKey.Close()

	// Set the name of the context menu item
	if err := shellKey.SetStringValue("", "Set as desktop background"); err != nil {
		return fmt.Errorf("failed to set shell registry value for %s: %v", ext, err)
	}

	commandKeyPath := shellKeyPath + `\command`
	commandKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, commandKeyPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create command registry key for %s: %v", ext, err)
	}
	defer commandKey.Close()

	// Command to run when the context menu item is selected
	if err := commandKey.SetStringValue("", exePath+` --set "%1"`); err != nil {
		return fmt.Errorf("failed to set command registry value for %s: %v", ext, err)
	}

	slog.Debug(fmt.Sprintf("Registry key set successfully for %s.\n", ext))
	return nil
}

func removeContextMenu(ext string) error {
	keyPath := fmt.Sprintf(`HKCR\SystemFileAssociations\%s\shell\SetAsDesktopBackground`, ext)
	cmd := exec.Command("reg", "delete", keyPath, "/f") // =)) why ???

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete registry key for %s: %v", ext, err)
	}
	slog.Debug(fmt.Sprintf("Registry key deleted successfully for %s.\n", ext))
	return nil
}

func displayHelp() {
	helpMessage := `
Usage: ElyWE [OPTIONS]

Options:
  --help                      Display help message
  --check                     Check if the mpv video player is installed correctly.
  --quit                      Kill all mpv processes
  --set <filePath>            Set video file path
  --install                   Install for right click menu (Requires Admin)
  --uninstall                 Uninstall for right click menu (Requires Admin)
  --enable_startup            Add shortcut to start with Windows
  --disable_startup           Remove startup shortcut
`
	fmt.Println(helpMessage)
}

func findMpv() (string, error) {
	return exec.LookPath("mpv")
}

/* Defender mark as trojan omg
func addToStartup(exePath string) error {
    // Open the registry key
    key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
    if err != nil {
        return err
    }
    defer key.Close()

    // Add the value to the registry
    err = key.SetStringValue(MyApp, exePath)
    if err != nil {
        return err
    }

    return nil
}

// removeFromStartup removes the application from startup
func removeFromStartup() error {
    // Open the registry key
    key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.WRITE)
    if err != nil {
        return err
    }
    defer key.Close()

    // Delete the value from the registry
    err = key.DeleteValue(MyApp)
    if err != nil {
        return err
    }

    return nil
}
*/

func getStartupPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("failed to get APPDATA environment variable")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

func createShortcut(targetPath, shortcutPath string) error {
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	shellObject, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("failed to create WScript.Shell object: %v", err)
	}
	defer shellObject.Release()

	shell, err := shellObject.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("failed to get IDispatch interface: %v", err)
	}
	defer shell.Release()

	shortcut, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
	if err != nil {
		return fmt.Errorf("failed to create shortcut: %v", err)
	}
	defer shortcut.Clear()

	sc := shortcut.ToIDispatch()

	oleutil.PutProperty(sc, "TargetPath", targetPath)
	oleutil.PutProperty(sc, "WorkingDirectory", filepath.Dir(targetPath))
	oleutil.PutProperty(sc, "WindowStyle", 1)
	oleutil.PutProperty(sc, "Description", "My Application")
	oleutil.PutProperty(sc, "IconLocation", targetPath+", 0")

	if _, err = oleutil.CallMethod(sc, "Save"); err != nil {
		return fmt.Errorf("failed to save shortcut: %v", err)
	}

	return nil
}

func addToStartup(targetPath string) error {
	startupPath, err := getStartupPath()
	if err != nil {
		return err
	}

	shortcutPath := filepath.Join(startupPath, MyApp+".lnk")
	return createShortcut(targetPath, shortcutPath)
}

func removeFromStartup() error {
	startupPath, err := getStartupPath()
	if err != nil {
		return err
	}

	shortcutPath := filepath.Join(startupPath, MyApp+".lnk")
	if _, err := os.Stat(shortcutPath); err != nil {
		return os.Remove(shortcutPath)
	} else if os.IsNotExist(err) {
		return fmt.Errorf("shortcut does not exist")
	} else {
		return err
	}
}

func main() {
	// just ascii art
	fmt.Println(" _____ _           _     __        _______ ")
	fmt.Println("| ____| |_   _ ___(_) __ \\ \\      / / ____|")
	fmt.Println("|  _| | | | | / __| |/ _` \\ \\ /\\ / /|  _|  ")
	fmt.Println("| |___| | |_| \\__ \\ | (_| |\\ V  V / | |___ ")
	fmt.Println("|_____|_|\\__, |___/_|\\__,_| \\_/\\_/  |_____|")
	fmt.Println("         |___/                             ")
	fmt.Println("")
	fmt.Println("Author	:", Author)
	fmt.Println("Version	:", Version)
	fmt.Println("")
	windowsVersion, _, _ := windows.RtlGetNtVersionNumbers()

	if windowsVersion < 8 {
		if err := showMessageBox("Error", "Windows version is less than Windows 8", MB_ICONERROR); err != nil {
			slog.Error("Windows version is less than Windows 8")
		}
		os.Exit(1)
	}

	help := flag.Bool("help", false, "Display help message")
	check := flag.Bool("check", false, "Check if the mpv video player is installed correctly.")
	quit := flag.Bool("quit", false, "Kill all mpv process")
	set := flag.String("set", "", "Set video file path")
	install := flag.Bool("install", false, "Install for right click menu (Required Admin)")
	uninstall := flag.Bool("uninstall", false, "Uninstall for right click menu (Required Admin)")
	enableStartup := flag.Bool("enable_startup", false, "Add registry key to start with Windows")
	disableStartup := flag.Bool("disable_startup", false, "Remove registry key to not start with Windows")
	findMpvTimeout := flag.Int("find_mpv_timeout", 5, "Timeout in seconds to find mpv window")
	verbose := flag.Bool("verbose", false, "Verbose logging")
	test := flag.Bool("test", false, "Test some stuff")

	flag.Parse()

	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	exePath, _, err := getExecPathAndDir()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get executable path: %v", err))
		showMessageBox("Error", "Failed to get executable path", MB_ICONERROR)
		os.Exit(1)
	}

	if *test {
		showMessageBox("Test", "read if cute <3", MB_ICONINFORMATION|MB_OK)
		os.Exit(0)
	}

	// Check registry
	checkKeyResult := checkKey()

	if checkKeyResult != 0 {
		switch checkKeyResult {
		case 1:
			{
				key, _, err := registry.CreateKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
				if err != nil {
					log.Fatal(err)
				}

				if err := key.SetStringValue("VideoPath", ""); err != nil {
					log.Fatal(err)
				}
				break
			}
		case 2, 3:
			{
				key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
				if err != nil {
					log.Fatal(err)
				}

				if err := key.SetStringValue("VideoPath", ""); err != nil {
					log.Fatal(err)
				}
				break
			}
		}
		return // Exit
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to open registry key: %v", err))
		showMessageBox("Error", "Failed to open registry key", MB_ICONERROR)
		os.Exit(1)
	}

	path, _, err := key.GetStringValue("VideoPath")
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to get registry key VideoPath: %v", err))
		// showMessageBox("Error", "Failed to get registry key", MB_ICONERROR)
		// os.Exit(1)
	}

	defer key.Close()
	// Args
	if *help || os.Args[1] == "" {
		displayHelp()
		os.Exit(0)
	}
	if *quit {
		if err := stopMPV(); err != nil {
			slog.Error(fmt.Sprintf("Failed to stop mpv: %v", err))
			showMessageBox("Error", "Failed to stop mpv", MB_ICONERROR)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *check {
		path, err := findMpv()
		if err != nil {
			fmt.Println("Error: Your system does not have a valid video player (mpv) installed.")
			fmt.Println("Please install it using the following command:")
			fmt.Println("$ choco install mpv")
			fmt.Println("If you have never used Chocolatey or installed a package with Chocolatey,")
			fmt.Println("please see the following guide: https://dev.to/stephanlamoureux/getting-started-with-chocolatey-epo")
			os.Exit(1)
		}

		slog.Info(fmt.Sprintf("Found mpv at %s", path))
		os.Exit(0)
	}
	if *install {
		if !isAdmin() {
			if err := runMeElevated(); err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
		for _, ext := range videoExtensions {
			if err := createContextMenu(ext, exePath); err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		}
		showMessageBox("Success", "Context menu entries created successfully for some video file types.", MB_ICONINFORMATION|MB_OK)
		os.Exit(0)
	}
	if *uninstall {
		if !isAdmin() {
			if err := runMeElevated(); err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
		for _, ext := range videoExtensions {
			if err := removeContextMenu(ext); err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
		}
		showMessageBox("Success", "Context menu entries removed successfully for some video file types.", MB_ICONINFORMATION|MB_OK)
		os.Exit(0)
	}
	if *set != "" {
		path = *set
		if path == "" {
			slog.Error("Video path is empty.")
			showMessageBox("Error", "Invalid path: "+path, MB_ICONERROR)
			os.Exit(1)
		}
		if err := checkPath(path); err != nil {
			slog.Error(fmt.Sprintf("checkPath error: %v", err))
			showMessageBox("Error", "Invalid path: "+path, MB_ICONERROR)
			os.Exit(1)
		}

		if err := key.SetStringValue("VideoPath", path); err != nil {
			slog.Error(fmt.Sprintf("Failed to set registry key: %v\n", err))
			showMessageBox("Error", "Failed to set registry key", MB_ICONERROR)
			os.Exit(1)
		}

		slog.Info("Set video wallpaper: " + path)

	}
	if *enableStartup {
		if err := addToStartup(exePath); err != nil {
			slog.Error(fmt.Sprintf("Failed to add to startup: %v", err))
			showMessageBox("Error", "Failed to add to startup", MB_ICONERROR)
			os.Exit(1)
		}
		showMessageBox("Success", "Successfully added to startup.", MB_ICONINFORMATION|MB_OK)
		os.Exit(0)
	}
	if *disableStartup {
		if err := removeFromStartup(); err != nil {
			slog.Error(fmt.Sprintf("Failed to remove from startup: %v", err))
			showMessageBox("Error", "Failed to remove from startup", MB_ICONERROR)
			os.Exit(1)
		}
		showMessageBox("Success", "Successfully removed from startup.", MB_ICONINFORMATION|MB_OK)
		os.Exit(0)
	}

	// End
	if path == "" {
		slog.Error("Video path is empty.")
		showMessageBox("Error", "Invalid path: "+path, MB_ICONERROR)
		os.Exit(1)
	}

	if err := stopMPV(); err != nil {
		if err.Error() == "no mpv processes found" {
			slog.Debug("No mpv processes found.")
		} else {
			slog.Error(fmt.Sprintf("Failed to stop mpv: %v", err))
			showMessageBox("Error", "Failed to stop mpv", MB_ICONERROR)
			os.Exit(1)
		}
	}

	if _, err := findMpv(); err != nil {
		slog.Error(fmt.Sprintf("mpv not found. error: %v", err))
		showMessageBox("Error", "mpv not found", MB_ICONERROR)
		os.Exit(1)
	}

	progman, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(UTF16PtrFromString(path))), 0)

	var result uintptr
	// Fix WorkerW not found: https://dynamicwallpaper.readthedocs.io/en/docs/dev/make-wallpaper.html#spawning-workerw
	procSendMessageTimeout.Call(progman, 0x052C, 0xD, 0, SMTO_NORMAL, 1000, uintptr(unsafe.Pointer(&result)))
	procSendMessageTimeout.Call(progman, 0x052C, 0xD, 1, SMTO_NORMAL, 1000, uintptr(unsafe.Pointer(&result)))

	var workerw uintptr
	enumWindowsCallback := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if shellView, _, _ := procFindWindowEx.Call(hwnd, 0, uintptr(unsafe.Pointer(UTF16PtrFromString("SHELLDLL_DefView"))), 0); shellView != 0 {
			workerw, _, _ = procFindWindowEx.Call(0, hwnd, uintptr(unsafe.Pointer(UTF16PtrFromString("WorkerW"))), 0)
		}
		return 1 // Continue enumeration
	})

	procEnumWindows.Call(enumWindowsCallback, 0)

	if workerw == 0 {
		slog.Error("WorkerW not found")
		showMessageBox("Error", "WorkerW not found", MB_ICONERROR)
	}

	slog.Debug("WorkerW found!")
	// Run MPV and get its window handle
	cmd := exec.Command("mpv", "--fs", "--loop", "--mute=yes", "--panscan=1.0", "--hwdec=auto", "--profile=low-latency", "--framedrop=no", "--scale=bilinear", "--dscale=bilinear", "--video-sync=display-resample", "--video-output-levels=full", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}

	if err := cmd.Start(); err != nil {
		slog.Debug(fmt.Sprintf("Failed to start MPV: %v", err))
		showMessageBox("Error", "Failed to start MPV", MB_ICONERROR)
		os.Exit(1)
	}

	// Wait for MPV window to appear
	var mpvWindow uintptr
	timeout := 0
	for {
		if timeout > *findMpvTimeout*10 {
			showMessageBox("Error", fmt.Sprintf("Failed to start MPV: Timeout (%ds)\nYour video might be invalid.", *findMpvTimeout), MB_ICONERROR)
			if err := key.SetStringValue("VideoPath", ""); err != nil {
				slog.Error(fmt.Sprintf("Failed to set registry key: %v", err))
			}
			os.Exit(1)
		}
		mpvWindow, _, _ = procFindWindow.Call(uintptr(unsafe.Pointer(UTF16PtrFromString("mpv"))), 0)
		if mpvWindow != 0 { // Found
			break
		}
		timeout++
		time.Sleep(100 * time.Millisecond)
	}

	// Get screen resolution
	screenWidth, _, err := procGetSystemMetrics.Call(SM_CXSCREEN)
	if err != nil && err.Error() != "The operation completed successfully." {
		slog.Error(fmt.Sprintf("Failed to get screen width: %v", err))
		os.Exit(1)
	}
	screenHeight, _, err := procGetSystemMetrics.Call(SM_CYSCREEN)
	if err != nil && err.Error() != "The operation completed successfully." {
		slog.Error(fmt.Sprintf("Failed to get screen height: %v", err))
		os.Exit(1)
	}

	// Set the MPV window position and size
	if _, _, err := procSetWindowPos.Call(mpvWindow, 0, 0, 0, screenWidth, screenHeight, SWP_NOZORDER); err != nil && err.Error() != "The operation completed successfully." {
		slog.Error(fmt.Sprintf("Failed to set MPV window position and size: %v", err))
		os.Exit(1)
	}

	// Set the MPV window as a child of WorkerW
	if _, _, err := procSetParent.Call(mpvWindow, workerw); err != nil && err.Error() != "The operation completed successfully." {
		slog.Error(fmt.Sprintf("Failed to set MPV window as child of WorkerW: %v", err))
		os.Exit(1)
	}

	slog.Debug("MPV window set as child of WorkerW.")
}
