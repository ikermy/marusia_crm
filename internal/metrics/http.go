package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func FiberMiddleware() fiber.Handler {
	Register()

	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		ObserveHTTPInFlight(1)
		defer ObserveHTTPInFlight(-1)

		err := c.Next()

		statusCode := c.Response().StatusCode()
		if err != nil && statusCode == fiber.StatusOK {
			statusCode = fiber.StatusInternalServerError
		}
		if statusCode == 0 {
			statusCode = fiber.StatusOK
		}

		routeName := c.Route().Path
		if routeName == "" {
			routeName = c.Path()
		}
		ObserveHTTP(c.Method(), routeName, strconv.Itoa(statusCode), startedAt)

		return err
	}
}
