package server

import "testing"

import "github.com/stretchr/testify/assert"
import "github.com/google/uuid"

// TestLogic dummy
func TestLogic(t *testing.T) {
	instance := Instance{}
	instance.Conf = &Config{}
	instance.Conf.Service.Address = "http://localhost:9090"
	instance.ID, _ = uuid.Parse("6e1bf099-7a5f-43a2-8cba-869bbc2e2ad5")

	healthURL := instance.CreateCheckURL()
	assert.Equal(t, "http://localhost:9090/health?id=6e1bf099-7a5f-43a2-8cba-869bbc2e2ad5", healthURL)
}
