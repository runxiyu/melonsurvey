package main

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	http.HandleFunc("/responses.csv", serveCSV)

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

	fmt.Fprintf(w, "感谢您的提交！您的数据已成功保存。")
}

func serveCSV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=responses.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	fieldOrder := []string{
		"gender", "age", "dialect", "wuyu_details", "guanhua_details", "other_details",
		"usage_frequency", "fluency", "foreign_language", "music_training",
		"absolute_pitch", "music_freq",
	}
	for i := 1; i <= 20; i++ {
		fieldOrder = append(fieldOrder, fmt.Sprintf("q%d", i))
	}
	fieldOrder = append(fieldOrder, "cadence1", "cadence2", "cadence3", "cadence4")
	fieldOrder = append(fieldOrder,
		"style1trap", "style2drill", "style3drumbass", "style4reggaetton", "style5rb",
	)

	if err := writer.Write(fieldOrder); err != nil {
		http.Error(w, "Failed to write header", http.StatusInternalServerError)
		return
	}

	files, err := os.ReadDir("responses")
	if err != nil {
		http.Error(w, "Failed to read responses", http.StatusInternalServerError)
		return
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		path := filepath.Join("responses", file.Name())

		f, err := os.Open(path)
		if err != nil {
			log.Printf("Failed to open file %s: %v", path, err)
			continue
		}

		var data map[string]string
		if err := json.NewDecoder(f).Decode(&data); err != nil {
			log.Printf("Failed to decode %s: %v", path, err)
			f.Close()
			continue
		}
		f.Close()

		var row []string
		for _, key := range fieldOrder {
			row = append(row, data[key])
		}
		if err := writer.Write(row); err != nil {
			log.Printf("Failed to write row: %v", err)
		}
	}
}
