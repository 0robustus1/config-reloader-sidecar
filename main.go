package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/go-ps"
	"golang.org/x/sys/unix"
)

func main() {
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		log.Fatal("mandatory env var CONFIG_DIR is empty, exiting")
	}

	processName := os.Getenv("PROCESS_NAME")
	processPIDFile := os.Getenv("PROCESS_PID_FILE")
	if processName != "" && processPIDFile != "" {
		log.Fatal("PROCESS_NAME and PROCESS_PID_FILE are mutually exclusive, exiting")
	}
	if processName == "" && processPIDFile == "" {
		log.Fatal("one of PROCESS_NAME or PROCESS_PID_FILE must be set, exiting")
	}

	verbose := false
	verboseFlag := os.Getenv("VERBOSE")
	if verboseFlag == "true" {
		verbose = true
	}

	var reloadSignal syscall.Signal
	reloadSignalStr := os.Getenv("RELOAD_SIGNAL")
	if reloadSignalStr == "" {
		log.Printf("RELOAD_SIGNAL is empty, defaulting to SIGHUP")
		reloadSignal = syscall.SIGHUP
	} else {
		reloadSignal = unix.SignalNum(reloadSignalStr)
		if reloadSignal == 0 {
			log.Fatalf("cannot find signal for RELOAD_SIGNAL: %s", reloadSignalStr)
		}
	}

	log.Printf("starting with CONFIG_DIR=%s, PROCESS_NAME=%s, PROCESS_PID_FILE=%s, RELOAD_SIGNAL=%s\n", configDir, processName, processPIDFile, reloadSignal)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if verbose {
					log.Println("event:", event)
				}
				if event.Op&fsnotify.Chmod != fsnotify.Chmod {
					log.Println("modified file:", event.Name)
					err := reloadProcess(processName, processPIDFile, reloadSignal)
					if err != nil {
						log.Println("error:", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	configDirs := strings.Split(configDir, ",")
	for _, dir := range configDirs {
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}
	}

	<-done
}

func findPID(process string) (int, error) {
	processes, err := ps.Processes()
	if err != nil {
		return -1, fmt.Errorf("failed to list processes: %v\n", err)
	}

	for _, p := range processes {
		if p.Executable() == process {
			log.Printf("found executable %s (pid: %d)\n", p.Executable(), p.Pid())
			return p.Pid(), nil
		}
	}

	return -1, fmt.Errorf("no process matching %s found\n", process)
}

func pidFromFile(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return -1, fmt.Errorf("failed to read pid file %s: %v", path, err)
	}

	pidString := strings.TrimSpace(string(content))
	if pidString == "" {
		return -1, fmt.Errorf("pid file %s is empty", path)
	}

	pid, err := strconv.Atoi(pidString)
	if err != nil {
		return -1, fmt.Errorf("pid file %s is not a valid integer: %v", path, err)
	}

	return pid, nil
}

func getPID(processName, pidFile string) (pid int, err error) {
	if pidFile != "" {
		return pidFromFile(pidFile)
	}
	return findPID(processName)
}

func reloadProcess(process string, pidFile string, signal syscall.Signal) error {
	var (
		pid int
		err error
	)

	pid, err = getPID(process, pidFile)
	if err != nil {
		return err
	}

	err = syscall.Kill(pid, signal)
	if err != nil {
		return fmt.Errorf("could not send signal: %v\n", err)
	}

	if pidFile != "" {
		log.Printf("signal %s sent to pid %d (from %s)\n", signal, pid, pidFile)
		return nil
	}

	log.Printf("signal %s sent to %s (pid: %d)\n", signal, process, pid)
	return nil
}
