package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	ChunkSize = 4 * 1024 * 1024
	MaxRetry  = 3
	Workers   = 3
)

type Chunk struct {
	Index int
	Data  []byte
}

func sha256Bytes(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

func fatal(msg string, err error) {
	if err != nil {
		fmt.Printf("\n❌ %s: %v\n", msg, err)
	} else {
		fmt.Printf("\n❌ %s\n", msg)
	}
	os.Exit(1)
}

type SSHFunc func(cmd string) (string, error)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage:")
		fmt.Println("  sshx-stream push user@host:port <local_dir> <remote_path>")
		fmt.Println("  sshx-stream pull user@host:port <remote_path> <local_dir>")
		os.Exit(1)
	}

	mode := os.Args[1]
	target := os.Args[2]
	localPath := os.Args[3]
	remotePath := os.Args[4]

	if !strings.Contains(target, "@") || !strings.Contains(target, ":") {
		fatal("Invalid target format", nil)
	}

	parts := strings.Split(target, "@")
	user := parts[0]
	hostParts := strings.Split(parts[1], ":")
	if len(hostParts) != 2 {
		fatal("Invalid host:port format", nil)
	}
	host := hostParts[0]
	port := hostParts[1]
	sshSocket := fmt.Sprintf("/tmp/sshx_mux_%s_%s_%s", user, host, port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("\n⚠ Interrupted. Cancelling...")
		cancel()
	}()

	sshCmd := func(cmd string) (string, error) {
		out, err := exec.Command("ssh",
			"-p", port,
			"-S", sshSocket,
			user+"@"+host,
			cmd).CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	if err := exec.Command("ssh",
		"-p", port,
		"-o", "ControlMaster=yes",
		"-o", "ControlPersist=600",
		"-o", "ControlPath="+sshSocket,
		"-fN",
		user+"@"+host).Run(); err != nil {
		fatal("SSH multiplex failed", err)
	}

	defer func() {
		exec.Command("ssh", "-S", sshSocket, "-O", "exit", user+"@"+host).Run()
	}()

	switch mode {
	case "push":
		push(ctx, localPath, remotePath, user, host, port, sshSocket, sshCmd)
	case "pull":
		pull(ctx, localPath, remotePath, user, host, port, sshSocket, sshCmd)
	default:
		fatal("Mode must be push or pull", nil)
	}
}

// ---------------- Push ----------------
func push(ctx context.Context, localDir, remotePath, user, host, port, sshSocket string, sshCmd SSHFunc) {
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		fatal("Directory not found", nil)
	}

	tmpTar := "/tmp/.sshx_" + filepath.Base(localDir) + ".tar.gz"
	defer os.Remove(tmpTar)

	fmt.Println("📦 Creating archive...")
	if err := exec.Command("tar", "-czf", tmpTar,
		"-C", filepath.Dir(localDir),
		filepath.Base(localDir)).Run(); err != nil {
		fatal("Tar failed", err)
	}

	info, _ := os.Stat(tmpTar)
	fileSize := info.Size()
	totalChunks := int((fileSize + ChunkSize - 1) / ChunkSize)

	if _, err := sshCmd(fmt.Sprintf("mkdir -p \"%s\"", remotePath)); err != nil {
		fatal("Remote mkdir failed", err)
	}

	remoteSizeStr, _ := sshCmd(fmt.Sprintf("stat -c%%s \"%s/.sshx_partial.tar.gz\" 2>/dev/null || echo 0", remotePath))
	remoteSize, _ := strconv.ParseInt(remoteSizeStr, 10, 64)

	if remoteSize > fileSize {
		fatal("Remote partial larger than local archive", nil)
	}
	if remoteSize%ChunkSize != 0 {
		fatal("Remote partial misaligned (corrupt resume)", nil)
	}
	startChunk := int(remoteSize / ChunkSize)

	file, err := os.Open(tmpTar)
	if err != nil {
		fatal("Open failed", err)
	}
	defer file.Close()
	file.Seek(int64(startChunk)*ChunkSize, io.SeekStart)

	chunkChan := make(chan Chunk, Workers)
	var wg sync.WaitGroup
	var transferred int64 = remoteSize
	startTime := time.Now()

	for w := 0; w < Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case chunk, ok := <-chunkChan:
					if !ok {
						return
					}
					localHash := sha256Bytes(chunk.Data)
					success := false
					for r := 0; r < MaxRetry; r++ {
						cmd := exec.CommandContext(ctx, "ssh",
							"-p", port,
							"-S", sshSocket,
							user+"@"+host,
							fmt.Sprintf("dd of=\"%s/.sshx_partial.tar.gz\" bs=%d seek=%d conv=notrunc",
								remotePath, ChunkSize, chunk.Index))
						cmd.Stdin = bytes.NewReader(chunk.Data)
						if err := cmd.Run(); err != nil {
							if ctx.Err() != nil {
								return
							}
							continue
						}

						remoteHash, err := sshCmd(fmt.Sprintf(
							"dd if=\"%s/.sshx_partial.tar.gz\" bs=%d skip=%d count=1 2>/dev/null | sha256sum | awk '{print $1}'",
							remotePath, ChunkSize, chunk.Index))
						if err == nil && localHash == remoteHash {
							success = true
							break
						}
					}

					if !success {
						fatal(fmt.Sprintf("Chunk %d failed", chunk.Index), nil)
					}
					atomic.AddInt64(&transferred, int64(len(chunk.Data)))

					// ✅ Show push progress
					elapsed := time.Since(startTime).Seconds()
					if elapsed > 0 {
						current := atomic.LoadInt64(&transferred)
						speed := float64(current) / elapsed / 1024
						percent := int(float64(current) / float64(fileSize) * 100)
						fmt.Printf("\r📊 %3d%% | ⚡ %.0f KB/s", percent, speed)
					}

				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// ---------------- Fixed ProducerLoop with progress ----------------
ProducerLoop:
	for i := startChunk; i < totalChunks; i++ {
		buffer := make([]byte, ChunkSize)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			fatal("Read error", err)
		}

		select {
		case <-ctx.Done():
			break ProducerLoop
		case chunkChan <- Chunk{Index: i, Data: buffer[:n]}:
			// display progress while producing
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				current := atomic.LoadInt64(&transferred)
				speed := float64(current) / elapsed / 1024
				percent := int(float64(current) / float64(fileSize) * 100)
				fmt.Printf("\r📊 %3d%% | ⚡ %.0f KB/s", percent, speed)
			}
		}
	}

	close(chunkChan)
	wg.Wait()

	if ctx.Err() != nil {
		fatal("Push cancelled", nil)
	}

	fmt.Println("\n🔍 Validating archive...")
	if _, err := sshCmd(fmt.Sprintf("tar -tzf \"%s/.sshx_partial.tar.gz\" > /dev/null", remotePath)); err != nil {
		fatal("Remote archive corrupted", err)
	}

	fmt.Println("📦 Extracting...")
	if _, err := sshCmd(fmt.Sprintf("tar -xzf \"%s/.sshx_partial.tar.gz\" -C \"%s\" && rm -f \"%s/.sshx_partial.tar.gz\"",
		remotePath, remotePath, remotePath)); err != nil {
		fatal("Extraction failed", err)
	}

	fmt.Println("✅ Push completed")
}

// ---------------- Pull ----------------
func pull(ctx context.Context, localDir, remotePath, user, host, port, sshSocket string, sshCmd SSHFunc) {
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		if err := os.MkdirAll(localDir, 0755); err != nil {
			fatal("Cannot create local directory", err)
		}
	}

	localTmp := filepath.Join(os.TempDir(), ".sshx_partial_pull.tar.gz")
	defer os.Remove(localTmp)

	remoteTmp := fmt.Sprintf("%s/.sshx_remote_tmp.tar.gz", remotePath)
	defer sshCmd(fmt.Sprintf("rm -f %s", remoteTmp))

	if _, err := sshCmd(fmt.Sprintf("tar -czf \"%s\" -C \"%s\" .", remoteTmp, remotePath)); err != nil {
		fatal("Remote tar creation failed", err)
	}

	sizeStr, err := sshCmd(fmt.Sprintf("stat -c%%s \"%s\"", remoteTmp))
	if err != nil {
		fatal("Remote stat failed", err)
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
	totalChunks := int((size + ChunkSize - 1) / ChunkSize)

	var startChunk int
	if info, err := os.Stat(localTmp); err == nil {
		if info.Size()%ChunkSize != 0 || info.Size() > size {
			os.Remove(localTmp)
			startChunk = 0
		} else {
			startChunk = int(info.Size() / ChunkSize)
		}
	}

	localFile, err := os.OpenFile(localTmp, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fatal("Cannot open local temp file", err)
	}
	defer localFile.Close()

	// ---------------- Safe sh-compatible hash prefetch ----------------
	fmt.Println("🔍 Fetching remote hashes...")
	remoteHashesRaw, err := sshCmd(fmt.Sprintf(`
i=%d
while [ $i -lt %d ]; do
	dd if="%s" bs=%d skip=$i count=1 2>/dev/null | sha256sum | awk '{print $1}'
	i=$((i+1))
done
`, startChunk, totalChunks, remoteTmp, ChunkSize))
	if err != nil {
		fatal("Failed to fetch remote hashes", err)
	}
	remoteHashes := strings.Split(strings.TrimSpace(remoteHashesRaw), "\n")
	if len(remoteHashes) != totalChunks-startChunk {
		fatal("Remote hash count mismatch", nil)
	}

	chunkChan := make(chan int, Workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var transferred int64 = int64(startChunk) * ChunkSize
	startTime := time.Now()

	for w := 0; w < Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case idx, ok := <-chunkChan:
					if !ok {
						return
					}

					success := false
					var chunkData []byte
					for r := 0; r < MaxRetry; r++ {
						out, err := exec.CommandContext(ctx, "ssh",
							"-p", port,
							"-S", sshSocket,
							user+"@"+host,
							fmt.Sprintf("dd if=\"%s\" bs=%d skip=%d count=1 2>/dev/null", remoteTmp, ChunkSize, idx)).Output()
						if err != nil {
							if ctx.Err() != nil {
								return
							}
							continue
						}

						if sha256Bytes(out) == strings.TrimSpace(remoteHashes[idx-startChunk]) {
							chunkData = out
							success = true
							break
						}
					}

					if !success {
						fatal(fmt.Sprintf("Chunk %d failed hash verification", idx), nil)
					}

					mu.Lock()
					localFile.Seek(int64(idx)*ChunkSize, io.SeekStart)
					localFile.Write(chunkData)
					atomic.AddInt64(&transferred, int64(len(chunkData)))
					mu.Unlock()

					elapsed := time.Since(startTime).Seconds()
					speed := float64(atomic.LoadInt64(&transferred)) / elapsed / 1024
					percent := int(float64(atomic.LoadInt64(&transferred)) / float64(size) * 100)
					fmt.Printf("\r📊 %3d%% | ⚡ %.0f KB/s", percent, speed)

				case <-ctx.Done():
					return
				}
			}
		}()
	}

ProducerLoop:
	for i := startChunk; i < totalChunks; i++ {
		select {
		case <-ctx.Done():
			break ProducerLoop
		case chunkChan <- i:
		}
	}

	close(chunkChan)
	wg.Wait()

	if ctx.Err() != nil {
		fatal("Pull cancelled", nil)
	}

	fmt.Println("\n📦 Extracting...")
	if err := exec.Command("tar", "-xzf", localTmp, "-C", localDir).Run(); err != nil {
		fatal("Local extraction failed", err)
	}
	fmt.Println("✅ Pull completed")
}
