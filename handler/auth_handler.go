package handler

import (
	"gosplitbill/service"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AuthHandler struct {
	svcAuth *service.AuthService
}

func NewAuthHandler(svcAuthc *service.AuthService) *AuthHandler {
	return &AuthHandler{svcAuth: svcAuthc}
}

// getCookieSecure membaca COOKIE_SECURE dari .env
func getCookieSecure() bool {
	return strings.ToLower(os.Getenv("COOKIE_SECURE")) == "true"
}

// getCookieSameSite membaca COOKIE_SAMESITE dari .env
func getCookieSameSite() string {
	sameSite := os.Getenv("COOKIE_SAMESITE")
	switch strings.ToLower(sameSite) {
	case "lax":
		return "Lax"
	case "none":
		return "None"
	case "strict":
		return "Strict"
	default:
		return "Strict"
	}
}

// getCookieDomain membaca COOKIE_DOMAIN dari .env
func getCookieDomain() string {
	return os.Getenv("COOKIE_DOMAIN")
}

// get user_id
func GetUserID(c *fiber.Ctx) uuid.UUID {
	return c.Locals("user_id").(uuid.UUID)
}


// endpoint : /api/v1/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var InputRegister struct {
		Email string `json:"email"`
		UserName string `json:"username"`
		Password string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}

	if err := c.BodyParser(&InputRegister); err != nil {
		return c.Status(401).JSON(fiber.Map{
			"success": false,
			"message": "Format tidak valid",
		})
	}

	_, err := h.svcAuth.Register(c.Context(), InputRegister.UserName, strings.ToLower(InputRegister.Email), InputRegister.Password, InputRegister.ConfirmPassword)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"messagge": err.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"success": true,
		"messagge": "Register Berhasil!, silahkan lakukan login",
	})
}

// enpoint : /api/v1/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var inputLogin struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&inputLogin); err != nil {
		return c.Status(401).JSON(fiber.Map{
			"success": false,
			"messagge": "Format tidak valid",
		})
	}

	accesToken, err := h.svcAuth.Login(c.Context(), inputLogin.Email, inputLogin.Password)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"messagge": err.Error(), 
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accesToken,
		MaxAge:   60 * 60 * 24,
		HTTPOnly: true,
		Secure:   getCookieSecure(),
		SameSite: getCookieSameSite(),
		Domain:   getCookieDomain(),
		Path: "/",
	})

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"messagge": "Login Berhasil",
	})
}

// endpoint : /api/v1/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {

    // Hapus kedua cookie
    c.Cookie(&fiber.Cookie{
        Name:     "access_token",
        Value:    "",
        MaxAge:   -1,
        HTTPOnly: true,
        Secure:   getCookieSecure(),
        SameSite: getCookieSameSite(),
        Domain:   getCookieDomain(),
		Path: "/",
    })

    return c.JSON(fiber.Map{
        "success": true,
        "messagge": "Logout berhasil",
    })
}