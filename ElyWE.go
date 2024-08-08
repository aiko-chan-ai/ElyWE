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

func stopMPV() {
	// Check if any mpv processes are running
	checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq mpv.exe")
	var out bytes.Buffer
	checkCmd.Stdout = &out
	err := checkCmd.Run()
	if err != nil {
		fmt.Printf("Failed to check for mpv processes: %v\n", err)
		return
	}
	if bytes.Contains(out.Bytes(), []byte("mpv.exe")) {
		killCmd := exec.Command("taskkill", "/F", "/IM", "mpv.exe")
		err := killCmd.Run()
		if err != nil {
			fmt.Printf("Failed to kill mpv processes: %v\n", err)
		} else {
			log.Println("mpv processes killed successfully.")
		}
	} else {
		log.Println("No mpv processes found.")
	}
}

func isValidPath(path string) bool {
	if path == "" {
		return true
	}
	if !filepath.IsAbs(path) {
		return false
	}
	file, err := os.Stat(path)
	if err != nil {
		return false
	}
	if file.IsDir() {
		return false
	}
	return true
}

func checkKey() (bool, int) {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.QUERY_VALUE)
	if err != nil {
		return false, 1 // Key not found
	}
	defer key.Close()
	val, _, err := key.GetStringValue("VideoPath")
	if err != nil {
		return false, 2 // VideoPath not found
	}
	if isValidPath(val) {
		return true, 0
	} else {
		return false, 3 // File invalid
	}
}

func getExecPathAndDir() (string, string) {
	exePath, err := os.Executable()
	if err != nil {
		log.Println("Error:", err)
		return "", ""
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		log.Println("Error:", err)
		return "", ""
	}
	exeDir := filepath.Dir(exePath)
	return exePath, exeDir
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func runMeElevated() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 1 //SW_NORMAL

	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
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

func showMessageBox(title, text string, style uintptr) int {
	ret, _, _ := messageBox.Call(
		0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(MyApp+" - "+title))),
		style,
	)

	return int(ret)
}

func createContextMenu(ext string, exePath string) {
	shellKeyPath := fmt.Sprintf(`SystemFileAssociations\%s\shell\SetAsDesktopBackground`, ext)
	shellKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, shellKeyPath, registry.ALL_ACCESS)
	if err != nil {
		log.Fatalf("Failed to create shell registry key for %s: %v", ext, err)
	}
	defer shellKey.Close()

	// Set the name of the context menu item
	err = shellKey.SetStringValue("", "Set as desktop background")
	if err != nil {
		log.Fatalf("Failed to set shell registry value for %s: %v", ext, err)
	}

	commandKeyPath := shellKeyPath + `\command`
	commandKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, commandKeyPath, registry.ALL_ACCESS)
	if err != nil {
		log.Fatalf("Failed to create command registry key for %s: %v", ext, err)
	}
	defer commandKey.Close()

	// Command to run when the context menu item is selected
	command := exePath + ` --set "%1"`

	err = commandKey.SetStringValue("", command)
	if err != nil {
		log.Fatalf("Failed to set command registry value for %s: %v", ext, err)
	}

	fmt.Printf("Registry key set successfully for %s.\n", ext)
}

func removeContextMenu(ext string) {
	keyPath := fmt.Sprintf(`HKCR\SystemFileAssociations\%s\shell\SetAsDesktopBackground`, ext)
	cmd := exec.Command("reg", "delete", keyPath, "/f") // =)) why ???
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to delete registry key for %s: %v", ext, err)
	} else {
		fmt.Printf("Registry key deleted successfully for %s.\n", ext)
	}
}

func displayHelp() {
	helpMessage := `
Usage: ElyWE [OPTIONS]

Options:
  -help                      Display help message
  -check                     Check if the mpv video player is installed correctly.
  -quit                      Kill all mpv processes
  -set <filePath>            Set video file path
  -install                   Install for right click menu (Requires Admin)
  -uninstall                 Uninstall for right click menu (Requires Admin)
  -enable_startup            Add shortcut to start with Windows
  -disable_startup           Remove startup shortcut
`
	fmt.Println(helpMessage)
}

func checkMpv() bool {
	_, err := exec.LookPath("mpv")
	if err != nil {
		fmt.Println("Error: Your system does not have a valid video player (mpv) installed.")
		fmt.Println("Please install it using the following command:")
		fmt.Println("$ choco install mpv")
		fmt.Println("If you have never used Chocolatey or installed a package with Chocolatey,")
		fmt.Println("please see the following guide: https://dev.to/stephanlamoureux/getting-started-with-chocolatey-epo")
		return false
	} else {
		log.Println("mpv is installed and in your PATH.")
		return true
	}
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

	_, err = oleutil.CallMethod(sc, "Save")
	if err != nil {
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
	if _, err := os.Stat(shortcutPath); err == nil {
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
	maj, _, _ := windows.RtlGetNtVersionNumbers()

	if (maj < 8) {
		showMessageBox("Error", "Windows version is less than Windows 8", MB_ICONERROR)
		return;
	}

	help := flag.Bool("help", false, "Display help message")
	check := flag.Bool("check", false, "Check if the mpv video player is installed correctly.")
	quit := flag.Bool("quit", false, "Kill all mpv process")
	set := flag.String("set", "", "Set video file path")
	install := flag.Bool("install", false, "Install for right click menu (Required Admin)")
	uninstall := flag.Bool("uninstall", false, "Uninstall for right click menu (Required Admin)")
	enableStartup := flag.Bool("enable_startup", false, "Add registry key to start with Windows")
	disableStartup := flag.Bool("disable_startup", false, "Remove registry key to not start with Windows")
	test := flag.Bool("test", false, "Test some stuff")

	flag.Parse()

	exePath, _ := getExecPathAndDir()

	if *test {
		showMessageBox("Test", "read if cute <3", MB_ICONINFORMATION|MB_OK)
		return
	}

	// Check registry
	stats, typeCheck := checkKey()
	var (
		key  registry.Key
		err  error
		path = ""
	)
	if !stats {
		switch typeCheck {
		case 1:
			{
				key, _, err = registry.CreateKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
				if err != nil {
					log.Fatal(err)
				}
				err = key.SetStringValue("VideoPath", path)
				if err != nil {
					log.Fatal(err)
				}
				break
			}
		case 2, 3:
			{
				key, err = registry.OpenKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
				if err != nil {
					log.Fatal(err)
				}
				err = key.SetStringValue("VideoPath", path)
				if err != nil {
					log.Fatal(err)
				}
				break
			}
		}
	} else {
		key, err = registry.OpenKey(registry.CURRENT_USER, "Software\\"+MyApp, registry.ALL_ACCESS)
		if err != nil {
			log.Fatal(err)
		}
		path, _, err = key.GetStringValue("VideoPath")
		if err != nil {
			log.Fatal(err)
		}
	}
	defer key.Close()
	// Args
	if *help {
		displayHelp()
		return
	} else if *quit {
		stopMPV()
		return
	} else if *check {
		_ = checkMpv()
		return
	} else if *install {
		if !isAdmin() {
			runMeElevated()
			return
		}
		for _, ext := range videoExtensions {
			createContextMenu(ext, exePath)
		}
		showMessageBox("Success", "Context menu entries created successfully for some video file types.", MB_ICONINFORMATION|MB_OK)
		return
	} else if *uninstall {
		if !isAdmin() {
			runMeElevated()
			return
		}
		for _, ext := range videoExtensions {
			removeContextMenu(ext)
		}
		showMessageBox("Success", "Context menu entries removed successfully for some video file types.", MB_ICONINFORMATION|MB_OK)
		return
	} else if *set != "" {
		path = *set
		if path == "" {
			showMessageBox("Error", "Invalid path: "+path, MB_ICONERROR)
			return
		} else {
			if isValidPath(path) {
				err = key.SetStringValue("VideoPath", path)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("Set video wallpaper: " + path)
			} else {
				showMessageBox("Error", "Invalid path: "+path, MB_ICONERROR)
				return
			}
		}
	} else if *enableStartup {
		addToStartup(exePath)
		showMessageBox("Success", "Successfully added to startup.", MB_ICONINFORMATION|MB_OK)
		return
	} else if *disableStartup {
		removeFromStartup()
		showMessageBox("Success", "Successfully removed from startup.", MB_ICONINFORMATION|MB_OK)
		return
	}

	// End
	if path == "" {
		log.Fatalln("Video path is empty.")
		return
	}
	stopMPV()
	if !checkMpv() {
		showMessageBox("Error", "mpv not found", MB_ICONERROR)
		return
	}
	progman, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Progman"))), 0)

	var result uintptr
	// Fix WorkerW not found: https://dynamicwallpaper.readthedocs.io/en/docs/dev/make-wallpaper.html#spawning-workerw
	procSendMessageTimeout.Call(progman, 0x052C, 0xD, 0, SMTO_NORMAL, 1000, uintptr(unsafe.Pointer(&result)))
	procSendMessageTimeout.Call(progman, 0x052C, 0xD, 1, SMTO_NORMAL, 1000, uintptr(unsafe.Pointer(&result)))

	var workerw uintptr
	enumWindowsCallback := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		shellView, _, _ := procFindWindowEx.Call(hwnd, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("SHELLDLL_DefView"))), 0)
		if shellView != 0 {
			workerw, _, _ = procFindWindowEx.Call(0, hwnd, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("WorkerW"))), 0)
		}
		return 1 // Continue enumeration
	})

	procEnumWindows.Call(enumWindowsCallback, 0)

	if workerw != 0 {
		log.Println("WorkerW found!")
		// Run MPV and get its window handle
		cmd := exec.Command("mpv", "--fs", "--loop", "--mute=yes", "--panscan=1.0", "--hwdec=auto", "--profile=low-latency", "--framedrop=no", "--scale=bilinear", "--dscale=bilinear", "--video-sync=display-resample", "--video-output-levels=full", path)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
		err := cmd.Start()
		if err != nil {
			showMessageBox("Error", "Failed to start MPV: \n"+err.Error(), MB_ICONERROR)
			return
		}

		// Wait for MPV window to appear
		var mpvWindow uintptr
		var timeout = 0
		for {
			if timeout > 50 {
				showMessageBox("Error", "Failed to start MPV: Timeout (5s)\nYour video might be invalid.", MB_ICONERROR)
				err := key.SetStringValue("VideoPath", "")
				if err != nil {
					log.Fatal(err)
				}
				return
			}
			mpvWindow, _, _ = procFindWindow.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("mpv"))), 0)
			if mpvWindow != 0 {
				break
			}
			timeout++
			time.Sleep(100 * time.Millisecond)
		}

		// Get screen resolution
		screenWidth, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
		screenHeight, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

		// Set the MPV window position and size
		procSetWindowPos.Call(mpvWindow, 0, 0, 0, screenWidth, screenHeight, SWP_NOZORDER)

		// Set the MPV window as a child of WorkerW
		procSetParent.Call(mpvWindow, workerw)
		log.Println("MPV window set as child of WorkerW.")
	} else {
		showMessageBox("Error", "WorkerW not found", MB_ICONERROR)
	}
}
