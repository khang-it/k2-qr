package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/ducnpdev/vietqr"
	"github.com/fogleman/gg"
	qrcode "github.com/skip2/go-qrcode"
)

// ================= EMBED STATIC =================

// BẮT BUỘC phải có thư mục public
//
//go:embed public/*
var embeddedFiles embed.FS

// ================= API =================

func qrContent(w http.ResponseWriter, r *http.Request) {
	bankBin := r.URL.Query().Get("bankBin")
	accountNo := r.URL.Query().Get("accountNo")
	receiverName := r.URL.Query().Get("receiverName")
	amount := r.URL.Query().Get("amount")
	desc := r.URL.Query().Get("desc")
	timeStr := r.URL.Query().Get("timeStamp")

	desc = addTimeStampToDesc(desc, timeStr)

	if bankBin == "" || accountNo == "" || receiverName == "" {
		http.Error(w, "Thiếu tham số", http.StatusBadRequest)
		return
	}

	content := GenerateQr(bankBin, accountNo, receiverName, amount, desc, "")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"content": content})
}

func qrImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Println("ParseMultipartForm:", err)
	}

	bankBin := r.FormValue("bankBin")
	accountNo := r.FormValue("accountNo")
	receiverName := r.FormValue("receiverName")
	amount := r.FormValue("amount")
	desc := r.FormValue("desc")
	sizeStr := r.FormValue("size")
	colorStr := r.FormValue("qrcolor")
	timeStr := r.FormValue("timeStamp")

	desc = addTimeStampToDesc(desc, timeStr)

	if bankBin == "" || accountNo == "" || receiverName == "" {
		http.Error(w, "Thiếu tham số", http.StatusBadRequest)
		return
	}

	size := 512
	if sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 2048 {
			size = s
		}
	}

	var qrColor color.Color = color.Black
	if colorStr != "" {
		qrColor = parseHexColor(colorStr)
	}

	content := GenerateQr(bankBin, accountNo, receiverName, amount, desc, "")

	qr, err := qrcode.New(content, qrcode.High)
	if err != nil {
		http.Error(w, "Không tạo được QR: "+err.Error(), http.StatusInternalServerError)
		return
	}
	qr.ForegroundColor = qrColor
	qrImg := qr.Image(size)

	file, handler, err := r.FormFile("avatar")
	if err == nil && handler != nil {
		tmpPath := filepath.Join(os.TempDir(), "orbit-qr_avatar_"+handler.Filename)
		tmpFile, errCreate := os.Create(tmpPath)
		if errCreate == nil {
			_, _ = io.Copy(tmpFile, file)
			_ = tmpFile.Close()
			_ = file.Close()

			avatarSize := int(float64(size) * 0.3)
			roundedLogo, errAdd := AddCircularAvatarToQR(qrImg, tmpPath, avatarSize, color.White, float64(size)/80.0)
			if errAdd == nil {
				qrImg = imaging.OverlayCenter(qrImg, roundedLogo, 1.0)
			}
			_ = os.Remove(tmpPath)
		}
	}

	w.Header().Set("Content-Type", "image/png")
	_ = png.Encode(w, qrImg)
}

// ================= QR LOGIC =================

func GenerateQr(bankBin, accountNo, receiverName, amount, desc, mcc string) string {
	req := vietqr.RequestGenerateViQR{
		MerchantAccountInformation: vietqr.MerchantAccountInformation{
			AccountNo: strings.TrimSpace(accountNo),
			AcqID:     strings.TrimSpace(bankBin),
		},
		TransactionAmount: strings.TrimSpace(amount),
		AdditionalDataFieldTemplate: vietqr.AdditionalDataFieldTemplate{
			Description: strings.TrimSpace(desc),
		},
		Mcc:          mcc,
		ReceiverName: strings.ToUpper(strings.TrimSpace(receiverName)),
	}

	return vietqr.GenerateViQR(req)
}

func addTimeStampToDesc(desc, timeStr string) string {
	if timeStr == "" {
		return desc
	}
	tsMs, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		tsMs = time.Now().UnixMilli()
	}
	t := time.UnixMilli(tsMs)
	formatted := t.Format("20060102150405")
	return fmt.Sprintf("%s|%s", desc, formatted)
}

// ================= IMAGE UTILS =================

func AddCircularAvatarToQR(qrImg image.Image, avatarPath string, size int, borderColor color.Color, borderWidth float64) (image.Image, error) {
	logoFile, err := os.Open(avatarPath)
	if err != nil {
		return qrImg, err
	}
	defer logoFile.Close()

	logo, _, err := image.Decode(logoFile)
	if err != nil {
		return qrImg, err
	}

	logo = imaging.Resize(logo, size, size, imaging.Lanczos)

	dc := gg.NewContext(size, size)
	dc.DrawCircle(float64(size)/2, float64(size)/2, float64(size)/2)
	dc.Clip()
	dc.DrawImage(logo, 0, 0)

	if borderWidth > 0 {
		dc.SetLineWidth(borderWidth)
		dc.SetColor(borderColor)
		dc.DrawCircle(float64(size)/2, float64(size)/2, float64(size)/2-borderWidth/2)
		dc.Stroke()
	}
	return dc.Image(), nil
}

func parseHexColor(s string) color.Color {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if len(s) != 6 {
		return color.Black
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}

// ================= SERVER (RENDER READY) =================

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fsys, err := fs.Sub(embeddedFiles, "public")
	if err != nil {
		log.Fatal("Không tìm thấy thư mục public/: ", err)
	}

	http.Handle("/", http.FileServer(http.FS(fsys)))
	http.HandleFunc("/orbit-qr/content", qrContent)
	http.HandleFunc("/orbit-qr", qrImage)

	log.Println("Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
