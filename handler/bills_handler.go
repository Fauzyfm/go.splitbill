package handler

import (
	"gosplitbill/service"
	"gosplitbill/model"
	"strings"

	"github.com/gofiber/fiber/v2"
)



type BillsHandler struct {
	billService *service.BillService
}

func NewBillsHandler(billService *service.BillService) *BillsHandler {
	return &BillsHandler{billService: billService}
}

func (h *BillsHandler) SendImageForOcr(c *fiber.Ctx) error {
	userid := GetUserID(c)
	image, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "File gambar wajib diunggah pada field file",
		})
	}
	if image.Size > service.MaxOCRUploadSize {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"success": false,
			"message": "Ukuran gambar maksimal 10 MB",
		})
	}

	parsed, err := h.billService.ScanReceipt(c.UserContext(), image, userid)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    parsed,
	})
}


func (h *BillsHandler) SaveBill(c *fiber.Ctx) error {
	var input model.BillAfterValidasi

	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Format Request tidak valid",
		})
	}

	input.Bill.UserID = GetUserID(c)
	input.Bill.Title = strings.TrimSpace(input.Bill.Title)

	if input.Bill.Title == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Title wajib diisi",
		})
	}

	if len(input.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Minimal harus ada 1 participant",
		})
	}

	result, err := h.billService.CreateBillAfterValidasi(c.Context(), &input)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
        "success": true,
        "data":    result,
    })
}

