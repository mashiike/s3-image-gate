package imagegate

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	s3svc          *s3.S3
	rekognitionSvc *rekognition.Rekognition
	router         *mux.Router
	bucket         string
	keyPrefix      string
}

func newHandler(
	sess *session.Session,
	bucket string, keyPrefix string,
	viewIndex bool,
) *Handler {
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONErrorResponse(w, r, http.StatusNotFound)
	})
	h := &Handler{
		s3svc:          s3.New(sess),
		rekognitionSvc: rekognition.New(sess),
		router:         router,
		keyPrefix:      keyPrefix,
		bucket:         bucket,
	}
	router.HandleFunc("/upload_image", h.uploadImage)
	if viewIndex {
		router.HandleFunc("/", h.index).Methods(http.MethodGet)
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[info] method:%s path:%s\n", r.Method, r.URL.Path)
	r = r.WithContext(context.Background())
	h.router.ServeHTTP(w, r)
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONErrorResponse(w, r, http.StatusMethodNotAllowed)
		return
	}
	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("image")
	if err != nil {
		log.Printf("[error] retrieving the image: %s\n", err.Error())
		writeJSONErrorResponse(w, r, http.StatusBadRequest)
		return
	}
	defer file.Close()
	ct := handler.Header.Get("Content-Type")
	var ext string
	switch ct {
	case "image/png":
		ext = "png"
	case "image/jpg", "image/jpeg":
		ext = "jpg"
	case "image/gif":
		writeJSONResponse(w, r, http.StatusBadRequest, map[string]interface{}{
			"status":  http.StatusBadRequest,
			"success": false,
			"detail":  "image/gif not supported.",
		})
		return
	default:
		log.Printf("[info] upload file Content-Type:%s\n", ct)
		writeJSONErrorResponse(w, r, http.StatusBadRequest)
		return
	}
	imageUUID, err := genImageUUID()
	if err != nil {
		log.Printf("[error] generate image uuid: %s\n", err.Error())
		writeJSONErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	s3Input := &s3.PutObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(filepath.Join(h.keyPrefix, imageUUID+"."+ext)),
		Body:   file,
	}
	s3Output, err := h.s3svc.PutObjectWithContext(r.Context(), s3Input)
	s3URL := fmt.Sprintf("s3://%s/%s", ptrString(s3Input.Bucket), ptrString(s3Input.Key))
	if err != nil {
		log.Printf("[error] upload image faild path:%s error:%s\n",
			s3URL,
			err.Error(),
		)
		writeJSONErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	log.Printf("[info] action:upload_image path:%s version:%s size:%d type:%s\n",
		s3URL,
		ptrString(s3Output.VersionId),
		handler.Size,
		ct,
	)
	rekoOutput, err := h.rekognitionSvc.DetectModerationLabelsWithContext(r.Context(), &rekognition.DetectModerationLabelsInput{
		Image: &rekognition.Image{
			S3Object: &rekognition.S3Object{
				Bucket:  s3Input.Bucket,
				Name:    s3Input.Key,
				Version: s3Output.VersionId,
			},
		},
	})
	if err != nil {
		log.Printf("[error] detect moderation labels faild path:%s error:%s\n",
			s3URL,
			err.Error(),
		)
		writeJSONErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	data := map[string]interface{}{
		"url":          s3URL,
		"version":      s3Output.VersionId,
		"size":         handler.Size,
		"content_type": ct,
		"result":       rekoOutput,
	}
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		log.Printf("[error] encode rekognition result faild path:%s error:%s\n",
			s3URL,
			err.Error(),
		)
		writeJSONErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	_, err = h.s3svc.PutObjectWithContext(r.Context(), &s3.PutObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(filepath.Join(h.keyPrefix, imageUUID+".json")),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		log.Printf("[error] upload rekognition result faild path:%s error:%s\n",
			s3URL,
			err.Error(),
		)
		writeJSONErrorResponse(w, r, http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, r, http.StatusCreated, map[string]interface{}{
		"status":            http.StatusCreated,
		"success":           true,
		"moderation_labels": rekoOutput.ModerationLabels,
	})
}

var genImageUUID = func() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

//go:embed assets/index.html
var indexPage []byte

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write(indexPage)
}

func writeJSONErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int) {
	writeJSONResponse(w, r, statusCode, map[string]interface{}{
		"status":  statusCode,
		"success": false,
		"detail":  http.StatusText(statusCode),
	})
}

func writeJSONResponse(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"status": 500, "success": false, "detail": "Internal Server Error"}`)
		return
	}
	w.WriteHeader(statusCode)
	w.Write(buf.Bytes())
}

func ptrString(str *string) string {
	if str == nil {
		return "-"
	}
	return *str
}
