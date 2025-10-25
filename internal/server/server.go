package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/claude-code-proxy/proxy/internal/config"
	"github.com/claude-code-proxy/proxy/internal/daemon"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// Start initializes and starts the HTTP server
func Start(cfg *config.Config) error {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          "Claude-Code-Proxy",
		AppName:               "Claude Code Proxy v1.0.0",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "*",
	}))
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// Root endpoint - proxy info
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Claude Code Proxy",
			"version": "1.0.0",
			"status":  "running",
			"config": fiber.Map{
				"openai_base_url": cfg.OpenAIBaseURL,
				"routing_mode":    getRoutingMode(cfg),
				"opus_model":      getOpusModel(cfg),
				"sonnet_model":    getSonnetModel(cfg),
				"haiku_model":     getHaikuModel(cfg),
			},
			"endpoints": fiber.Map{
				"health":       "/health",
				"messages":     "/v1/messages",
				"count_tokens": "/v1/messages/count_tokens",
			},
		})
	})

	// Claude API endpoints
	setupClaudeEndpoints(app, cfg)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n🛑 Shutting down...")
		daemon.Cleanup()
		app.Shutdown()
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	fmt.Printf("✅ Proxy running at http://localhost:%s\n", cfg.Port)

	if cfg.PassthroughMode {
		fmt.Printf("   Mode: PASSTHROUGH (direct to Anthropic API)\n")
	} else {
		fmt.Printf("   Mode: Conversion (via %s)\n", cfg.OpenAIBaseURL)
		fmt.Printf("   Model Routing: %s\n", getRoutingMode(cfg))

		// Show actual model mappings
		if cfg.OpusModel != "" || cfg.SonnetModel != "" || cfg.HaikuModel != "" {
			fmt.Printf("   Models:\n")
			if cfg.OpusModel != "" {
				fmt.Printf("     - Opus   → %s\n", cfg.OpusModel)
			}
			if cfg.SonnetModel != "" {
				fmt.Printf("     - Sonnet → %s\n", cfg.SonnetModel)
			}
			if cfg.HaikuModel != "" {
				fmt.Printf("     - Haiku  → %s\n", cfg.HaikuModel)
			}
		}
	}

	return app.Listen(addr)
}

func getRoutingMode(cfg *config.Config) string {
	if cfg.OpusModel != "" || cfg.SonnetModel != "" || cfg.HaikuModel != "" {
		return "custom (env overrides)"
	}
	return "pattern-based"
}

func getOpusModel(cfg *config.Config) string {
	if cfg.OpusModel != "" {
		return cfg.OpusModel
	}
	return "gpt-5 (pattern-based)"
}

func getSonnetModel(cfg *config.Config) string {
	if cfg.SonnetModel != "" {
		return cfg.SonnetModel
	}
	return "version-aware (pattern-based)"
}

func getHaikuModel(cfg *config.Config) string {
	if cfg.HaikuModel != "" {
		return cfg.HaikuModel
	}
	return "gpt-5-mini (pattern-based)"
}

func setupClaudeEndpoints(app *fiber.App, cfg *config.Config) {
	// Messages endpoint - main Claude API
	app.Post("/v1/messages", func(c *fiber.Ctx) error {
		return handleMessages(c, cfg)
	})

	// Token counting endpoint
	app.Post("/v1/messages/count_tokens", func(c *fiber.Ctx) error {
		return handleCountTokens(c, cfg)
	})
}
