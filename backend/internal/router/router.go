package router

import (
	"github.com/gin-gonic/gin"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/handler"
	"github.com/gbexam/online-exam/internal/middleware"
)

// New assembles the HTTP routes and middleware chain.
func New(s *handler.Server, authMiddleware gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLog(s.Logger()))
	r.Use(middleware.CORS("*"))

	r.GET("/healthz", s.Health)
	r.GET("/health", s.Health)

	api := r.Group("/api/v1")
	{
		api.POST("/auth/register", s.Register)
		api.POST("/auth/login", s.Login)

		authorized := api.Group("")
		authorized.Use(authMiddleware)
		{
			authorized.GET("/auth/profile", s.Profile)
			authorized.POST("/auth/logout", s.Logout)

			adminOnly := authorized.Group("")
			adminOnly.Use(middleware.RequireRole(constants.RoleAdmin))
			{
				adminOnly.GET("/users", s.ListUsers)
				adminOnly.PUT("/users/:id/status", s.UpdateUserStatus)
			}

			staff := authorized.Group("")
			staff.Use(middleware.RequireRole(constants.RoleAdmin, constants.RoleTeacher))
			{
				staff.GET("/questions", s.ListQuestions)
				staff.POST("/questions", s.CreateQuestion)
				staff.GET("/questions/:id", s.GetQuestion)
				staff.PUT("/questions/:id", s.UpdateQuestion)
				staff.DELETE("/questions/:id", s.DeleteQuestion)
				staff.POST("/questions/batch-import", s.BatchImportQuestions)

				staff.POST("/exams", s.CreateExam)
				staff.GET("/exams/:id/questions", s.ListExamQuestions)
				staff.GET("/exams/:id/stats", s.ExamStats)
				staff.GET("/exams/:id/attempts", s.ListGrading)
				staff.POST("/exams/:id/publish", s.PublishExam)
				staff.POST("/exams/:id/close", s.CloseExam)
				staff.DELETE("/exams/:id", s.DeleteExam)
				staff.PUT("/attempts/:id/grade", s.GradeAttempt)
			}

			studentOnly := authorized.Group("")
			studentOnly.Use(middleware.RequireRole(constants.RoleStudent))
			{
				studentOnly.POST("/exams/:id/attempts", s.StartAttempt)
				studentOnly.GET("/exams/:id/attempts/current", s.CurrentAttempt)
				studentOnly.GET("/attempts", s.ListAttempts)
				studentOnly.POST("/attempts/:id/answers", s.SaveAnswer)
				studentOnly.POST("/attempts/:id/submit", s.SubmitAttempt)
				studentOnly.GET("/attempts/:id", s.GetAttemptDetail)
				studentOnly.GET("/attempts/:id/report", s.GetAttemptReport)
				studentOnly.GET("/wrong-questions", s.ListWrongQuestions)
				studentOnly.DELETE("/wrong-questions/:id", s.DeleteWrongQuestion)
				studentOnly.GET("/wrong-questions/practice", s.PracticeWrongQuestions)
				studentOnly.POST("/wrong-questions/practice", s.SubmitPractice)
			}

			authorized.GET("/exams", s.ListExams)
			authorized.GET("/exams/:id", s.GetExam)
			authorized.GET("/stats/overview", s.Overview)
		}
	}

	return r
}
