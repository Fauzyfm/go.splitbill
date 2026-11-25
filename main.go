package main

import (
	"errors"
	"gosplitbill/config"
	"gosplitbill/handler"
	"gosplitbill/middleware"
	"gosplitbill/repository"
	"gosplitbill/service"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {

	// check keberadaan file .env
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] File .env tidak ditemukan, menggunakan environment variable sistem")
	}

	// connect db
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("Gagal connect DB: %v", err)
	}

	// Jalankan Server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}


	app := fiber.New(fiber.Config{
		AppName: "Go Split Bill v1.0",

		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}

			messagge := err.Error()
			if code == fiber.StatusInternalServerError {
				log.Printf("[ERROR] internal: %v", err)
				messagge = "Terjadi kesalahan pada server"
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"messagge": messagge,
			})
		},

		BodyLimit: service.MaxOCRUploadSize + (64 << 10),
	})


	// Log startup info yang aman (tidak print secrets)
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}
	log.Println("Status db :", db.Stats())

	// Auth Depedency
	authRepo := repository.NewPostgraseAuthRepository(db)
	authService := service.NewAuthService(authRepo)
	authHandler := handler.NewAuthHandler(authService)

	// Participant Depedency
	participantRepo := repository.NewParticipantRepository(db)
	

	// Bill Item Depedency
	billItemRepo := repository.NewBillItemRepository(db)

	// Item Assignment Depedency
	assignmentRepo := repository.NewItemAssignMentRepository(db)

	// Bills Depedency
	billRepo := repository.NewPostgraseBillsRepository(db)
	billService := service.NewBillService(billRepo, billItemRepo, participantRepo, assignmentRepo)
	billHandler := handler.NewBillsHandler(billService)


	// api
	api := app.Group("/api/v1")

	// auth api
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)
	

	// bill api
	bill := api.Group("/bill", middleware.AuthRequired)
	bill.Post("/ocr", billHandler.SendImageForOcr)
	bill.Post("/save-bill", billHandler.SaveBill)



	log.Printf("Menjalankan server di port %s [env: %s]", port, appEnv)
	log.Fatal(app.Listen(":" + port))
}
