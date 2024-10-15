package pcm

import (
	"io"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	log "github.com/sirupsen/logrus" // logrus package
)

/*
asynchronous function that makes a HTTP request to the intel PCM
sensor server to retrieve sensor data.
*/
func MakePCMRequest(url string, filename string, wg *sync.WaitGroup, readyChan chan struct{}) {
	defer wg.Done()

	var mask uintptr

	// Get the current CPU affinity of the process
	if _, _, err := syscall.RawSyscall(syscall.SYS_SCHED_GETAFFINITY, 0, uintptr(unsafe.Sizeof(mask)), uintptr(unsafe.Pointer(&mask))); err != 0 {
		log.Println("Failed to get CPU affinity:", err)
		return
	}
	log.Println("Current CPU affinity:", mask)

	// Set the new CPU affinity
	mask = 0
	if _, _, err := syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(unsafe.Sizeof(mask)), uintptr(unsafe.Pointer(&mask))); err != 0 {
		log.Println("Failed to set CPU affinity:", err)
		return
	}
	log.Println("New CPU affinity:", mask)

	text := ""

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("Error creating request for %s: %s", url, err)
		readyChan <- struct{}{}
		return
	}

	// Add the custom header
	req.Header.Add("Accept", "application/json")

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 1 * time.Second}

	// Send the first request for metrics
	body, err := HandleClientReq(url, req, client)
	if err != nil {
		readyChan <- struct{}{}
		return
	}
	text += string(body)
	readyChan <- struct{}{}

	// TODO: ensure channels are handled more safely
	// wait for ROI to finish
	<-readyChan
	// Send the second request
	body, err = HandleClientReq(url, req, client)
	if err != nil {
		return
	}
	text += string(body)

	//write to string
	WriteToFile(filename, text)

}

func HandleClientReq(url string, req *http.Request, client *http.Client) (res string, err error) {
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Error sending HTTP request.", err)
		return "", err
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error reading HTTP response. ", err)
		return "", err
	}
	resp.Body.Close()
	return string(body), nil
}

func WriteToFile(filename string, text string) {
	//write to string
	f, err := os.Create(filename)
	if err != nil {
		log.Errorf("Error creating file %s. %s", filename, err)
		return
	}
	defer f.Close()
	nbytes, err := f.WriteString(text)
	if err != nil {
		log.Error("Failed to write text to file! ", err)
		return
	}
	log.Infof("Wrote %d bytes to file %s.\n", nbytes, filename)

}
