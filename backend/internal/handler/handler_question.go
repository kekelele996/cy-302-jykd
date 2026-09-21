package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/middleware"
	"github.com/gbexam/online-exam/pkg/httpx"
)

// ListQuestions handles GET /questions.
func (s *Server) ListQuestions(c *gin.Context) {
	var query dto.QuestionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.questions.List(c.Request.Context(), query)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

// GetQuestion handles GET /questions/:id.
func (s *Server) GetQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "题目 ID 不合法")
		return
	}
	question, err := s.questions.Get(c.Request.Context(), uint(id))
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, question)
}

// CreateQuestion handles POST /questions.
func (s *Server) CreateQuestion(c *gin.Context) {
	var req dto.QuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	question, err := s.questions.Create(c.Request.Context(), middleware.UserID(c), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.Created(c, question)
}

// UpdateQuestion handles PUT /questions/:id.
func (s *Server) UpdateQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "题目 ID 不合法")
		return
	}
	var req dto.QuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	question, err := s.questions.Update(c.Request.Context(), uint(id), req)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, question)
}

// DeleteQuestion handles DELETE /questions/:id.
func (s *Server) DeleteQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "题目 ID 不合法")
		return
	}
	if err := s.questions.Delete(c.Request.Context(), uint(id)); err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "删除成功"})
}

// BatchImportQuestions handles POST /questions/batch-import (JSON body or file upload).
func (s *Server) BatchImportQuestions(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		s.batchImportFile(c)
		return
	}

	var items []dto.QuestionRequest
	if err := c.ShouldBindJSON(&items); err != nil {
		httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "请求参数不合法")
		return
	}
	result, err := s.questions.ImportJSON(c.Request.Context(), middleware.UserID(c), items)
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (s *Server) batchImportFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "缺少上传文件")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		s.respondError(c, err)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	var result dto.BatchImportResult
	switch ext {
	case ".json":
		data, readErr := io.ReadAll(file)
		if readErr != nil {
			s.respondError(c, readErr)
			return
		}
		var items []dto.QuestionRequest
		if unmarshalErr := json.Unmarshal(data, &items); unmarshalErr != nil {
			httpx.Fail(c, http.StatusUnprocessableEntity, constants.CodeValidation, "JSON 文件格式不合法")
			return
		}
		result, err = s.questions.ImportJSON(c.Request.Context(), middleware.UserID(c), items)
	case ".xlsx":
		result, err = s.questions.ImportExcel(c.Request.Context(), middleware.UserID(c), file)
	default:
		httpx.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "仅支持 .json 或 .xlsx 文件")
		return
	}
	if err != nil {
		s.respondError(c, err)
		return
	}
	httpx.OK(c, result)
}
