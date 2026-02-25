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

// ================= SECURITY MIDDLEWARE =================

var allowedOrigins = map[string]bool{
	"https://wh.io.vn": true,
	// "https://localhost": true,
	// "http://localhost":  true, // dev local
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		origin := r.Header.Get("Origin")
		referer := r.Header.Get("Referer")

		_ = referer

		// ================= FRAME PROTECTION (IFRAME CONTROL) =================
		// Chỉ cho phép iframe từ wh.io.vn và localhost
		w.Header().Set(
			"Content-Security-Policy",
			"frame-ancestors https://wh.io.vn https://localhost http://localhost;",
		)

		// ================= CORS (API SECURITY) =================
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// ================= ENTERPRISE SECURITY HEADERS =================
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN") // fallback browser cũ
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// HSTS (chỉ bật khi chạy HTTPS production)
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}

func strictDomainGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		referer := r.Header.Get("Referer")
		host := r.Host

		_ = host

		// Cho phép health check hoặc direct access nếu cần
		// Nếu muốn CHẶN luôn truy cập trực tiếp -> bật dòng dưới
		if origin == "" && referer == "" {
			http.Error(w, "Forbidden - Direct access not allowed", http.StatusForbidden)
			return
		}

		// Kiểm tra Origin
		if origin != "" && allowedOrigins[origin] {
			next.ServeHTTP(w, r)
			return
		}

		// Kiểm tra Referer (iframe thường có referer)
		for allowed := range allowedOrigins {
			if strings.HasPrefix(referer, allowed) {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden - Invalid domain", http.StatusForbidden)
	})
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
	/*
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

		// Wrap với security middleware
		handler := securityHeaders(mux)

		log.Println("Secure Server running on port:", port)
		log.Fatal(http.ListenAndServe(":"+port, handler))

		log.Println("Server running on port:", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	*/
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fsys, err := fs.Sub(embeddedFiles, "public")
	if err != nil {
		log.Fatal("Không tìm thấy thư mục public/: ", err)
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/", http.FileServer(http.FS(fsys)))

	// API routes
	mux.HandleFunc("/orbit-qr/content", qrContent)
	mux.HandleFunc("/orbit-qr", qrImage)

	// Wrap với security middleware
	handler := securityHeaders(mux)
	//handler := strictDomainGuard(mux)

	log.Println("Secure Server running on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
