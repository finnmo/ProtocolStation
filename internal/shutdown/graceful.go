package shutdown

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// GracefulShutdown handles graceful shutdown of the bridge
type GracefulShutdown struct {
	logger       *zap.Logger
	components   []Shutdownable
	drainTimeout time.Duration
}

// Shutdownable interface for components that can be shut down gracefully
type Shutdownable interface {
	Shutdown(ctx context.Context) error
	Name() string
}

// New creates a new GracefulShutdown manager
func New(logger *zap.Logger, drainTimeout time.Duration) *GracefulShutdown {
	return &GracefulShutdown{
		logger:       logger,
		components:   make([]Shutdownable, 0),
		drainTimeout: drainTimeout,
	}
}

// Register registers a component for graceful shutdown
func (gs *GracefulShutdown) Register(component Shutdownable) {
	gs.components = append(gs.components, component)
	gs.logger.Debug("registered component for graceful shutdown",
		zap.String("component", component.Name()))
}

// Shutdown gracefully shuts down all registered components
func (gs *GracefulShutdown) Shutdown(ctx context.Context) error {
	gs.logger.Info("initiating graceful shutdown",
		zap.Int("components", len(gs.components)),
		zap.Duration("timeout", gs.drainTimeout))

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, gs.drainTimeout)
	defer cancel()

	// Shutdown all components in reverse order (cleanup first)
	errChan := make(chan error, len(gs.components))
	for i := len(gs.components) - 1; i >= 0; i-- {
		component := gs.components[i]
		go func(c Shutdownable) {
			gs.logger.Info("shutting down component",
				zap.String("component", c.Name()))

			if err := c.Shutdown(shutdownCtx); err != nil {
				gs.logger.Error("component shutdown failed",
					zap.String("component", c.Name()),
					zap.Error(err))
				errChan <- err
			} else {
				gs.logger.Info("component shut down successfully",
					zap.String("component", c.Name()))
				errChan <- nil
			}
		}(component)
	}

	// Wait for all shutdowns to complete or timeout
	var lastError error
	for i := 0; i < len(gs.components); i++ {
		if err := <-errChan; err != nil {
			lastError = err
		}
	}

	if lastError != nil {
		gs.logger.Warn("some components failed to shutdown gracefully",
			zap.Error(lastError))
	} else {
		gs.logger.Info("graceful shutdown completed successfully")
	}

	return lastError
}

// Drain waits for in-flight messages to complete
func (gs *GracefulShutdown) Drain(ctx context.Context, drainFunc func(context.Context) error) error {
	gs.logger.Info("draining in-flight messages")

	drainCtx, cancel := context.WithTimeout(ctx, gs.drainTimeout)
	defer cancel()

	if drainFunc != nil {
		return drainFunc(drainCtx)
	}

	// Default: just wait for drain timeout
	<-drainCtx.Done()
	gs.logger.Info("drain completed")
	return nil
}

// Component represents a component that can be registered for graceful shutdown
type Component struct {
	name         string
	shutdownFunc func(context.Context) error
}

// NewComponent creates a new component wrapper
func NewComponent(name string, shutdownFunc func(context.Context) error) *Component {
	return &Component{
		name:         name,
		shutdownFunc: shutdownFunc,
	}
}

// Shutdown calls the shutdown function
func (c *Component) Shutdown(ctx context.Context) error {
	if c.shutdownFunc != nil {
		return c.shutdownFunc(ctx)
	}
	return nil
}

// Name returns the component name
func (c *Component) Name() string {
	return c.name
}

