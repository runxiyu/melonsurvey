package main

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed static/*
var embeddedStatic embed.FS

func main() {
	fs, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		panic(err)
	}

	http.Handle("/", http.FileServer(http.FS(fs)))
	http.HandleFunc("/submit", handleForm)
	http.HandleFunc("/fwdaiusyflaidsunfuoiawenufwylnfkalhjdslkjfhjlwadk.csv", handleCSV)

	if err := os.MkdirAll("responses", 0755); err != nil {
		log.Fatalf("unable to create responses folder: %v", err)
	}

	log.Println("listening 127.0.0.1:9074")
	log.Fatal(http.ListenAndServe("127.0.0.1:9074", nil))
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	data := make(map[string]string)
	for key, values := range r.PostForm {
		if len(values) > 0 {
			data[key] = values[0]
		} else {
			data[key] = ""
		}
	}

	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.Split(ip, ",")[0]
		ip = strings.TrimSpace(ip)
	} else {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	data["ip_address"] = ip

	filename := time.Now().Format("20060102_150405.000") + ".json"
	filePath := filepath.Join("responses", filename)

	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "无法打开保存文件："+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")
	if err := encoder.Encode(data); err != nil {
		http.Error(w, "无法写入保存文件："+err.Error(), http.StatusInternalServerError)
		return
	}

	go func(data map[string]string) {
		cmd := "/sbin/sendmail"
		args := []string{"-t", "-i"}
		bodyBuilder := &strings.Builder{}
		fmt.Fprintf(bodyBuilder, "From: tiffany@runxiyu.org\n")
		fmt.Fprintf(bodyBuilder, "To: tiffany@runxiyu.org\n")
		fmt.Fprintf(bodyBuilder, "Subject: Survey response from %s\n", data["ip_address"])
		fmt.Fprintf(bodyBuilder, "MIME-Version: 1.0\n")
		fmt.Fprintf(bodyBuilder, "Content-Type: text/plain; charset=UTF-8\n")
		fmt.Fprintf(bodyBuilder, "Content-Transfer-Encoding: 8bit\n\n")

		jsonBytes, err := json.MarshalIndent(data, "", "\t")
		if err == nil {
			bodyBuilder.Write(jsonBytes)

			sendmail := exec.Command(cmd, args...)
			stdin, err := sendmail.StdinPipe()
			if err == nil {
				if err := sendmail.Start(); err == nil {
					stdin.Write([]byte(bodyBuilder.String()))
					stdin.Close()
					sendmail.Wait()
				}
			}
		}
	}(data)

	fmt.Fprintf(w, `恭喜您完成所有测试并衷心感谢您的参与！
如有兴趣了解实验数据分析结果，请关注微信公众号 @WIT studio。`)
}

func handleCSV(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir("responses")
	if err != nil {
		http.Error(w, "无法读取 responses 目录："+err.Error(), http.StatusInternalServerError)
		return
	}

	records := []map[string]string{}
	fieldSet := make(map[string]struct{})

	// First pass: collect all unique field names
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			content, err := ioutil.ReadFile(filepath.Join("responses", file.Name()))
			if err != nil {
				continue
			}

			var data map[string]string
			if err := json.Unmarshal(content, &data); err != nil {
				continue
			}

			for k := range data {
				fieldSet[k] = struct{}{}
			}
			records = append(records, data)
		}
	}
	fieldSet["Time"] = struct{}{}

	// Build field list (columns)
	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	// Prepare CSV buffer
	var buf bytes.Buffer
	buf.WriteString("\uFEFF") // UTF-8 BOM

	writer := csv.NewWriter(&buf)
	writer.Write(fields)

	for _, record := range records {
		row := make([]string, len(fields))
		for i, field := range fields {
			if field == "Time" {
			} else {
				row[i] = record[field]
			}
		}
		writer.Write(row)
	}
	writer.Flush()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"responses.csv\"")
	w.Write(buf.Bytes())
}
