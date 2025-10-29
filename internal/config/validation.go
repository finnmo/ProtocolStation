package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
	Hint    string
}

func (e *ValidationError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("validation error in %s: %s\n  Hint: %s", e.Field, e.Message, e.Hint)
	}
	return fmt.Sprintf("validation error in %s: %s", e.Field, e.Message)
}

// Validator holds validation state
type Validator struct {
	errs []error
}

func (v *Validator) addError(field string, value interface{}, message, hint string) {
	v.errs = append(v.errs, &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Hint:    hint,
	})
}

func (v *Validator) hasErrors() bool {
	return len(v.errs) > 0
}

func (v *Validator) getErrors() []error {
	return v.errs
}

// validateConfig performs comprehensive validation of the configuration
func validateConfig(cfg *Config) error {
	v := &Validator{}

	// Collect all names
	inputNames := make(map[string]bool)
	transformerNames := make(map[string]bool)
	outputNames := make(map[string]bool)
	clientIDs := make(map[string]bool)

	// Validate inputs
	for i, input := range cfg.Inputs {
		field := fmt.Sprintf("inputs[%d].%s", i, input.Name)

		if input.Name == "" {
			v.addError(field, input.Name, "input name is required", "")
			continue
		}

		// Check for duplicate names
		if inputNames[input.Name] {
			v.addError(field, input.Name, "duplicate input name", fmt.Sprintf("Input '%s' is defined multiple times", input.Name))
			continue
		}
		inputNames[input.Name] = true

		// Validate broker URL
		if err := validateBrokerURL(input.Broker, field+".broker"); err != nil {
			v.addError(field+".broker", input.Broker, err.Error(), "Expected format: 'protocol://host:port' or 'host:port'")
		}

		// Validate client ID
		if input.ClientID != "" {
			if err := validateClientID(input.ClientID); err != nil {
				v.addError(field+".client_id", input.ClientID, err.Error(), "Client ID must be alphanumeric with hyphens and underscores")
			}

			// Check for duplicate client IDs
			if clientIDs[input.ClientID] {
				v.addError(field+".client_id", input.ClientID, "duplicate client ID", fmt.Sprintf("Client ID '%s' is used by multiple components", input.ClientID))
			}
			clientIDs[input.ClientID] = true
		}

		// Validate topics
		if len(input.Topics) == 0 {
			v.addError(field+".topics", input.Topics, "at least one topic is required", "")
		}
		for j, topic := range input.Topics {
			if err := validateTopic(topic); err != nil {
				v.addError(fmt.Sprintf("%s.topics[%d]", field, j), topic, err.Error(), "")
			}
		}

		// Validate QoS
		if input.QoS < 0 || input.QoS > 2 {
			v.addError(field+".qos", input.QoS, "QoS must be between 0 and 2", "Valid values: 0 (at most once), 1 (at least once), 2 (exactly once)")
		}
	}

	// Validate transformers
	for i, transformer := range cfg.Transformers {
		field := fmt.Sprintf("transformers[%d].%s", i, transformer.Name)

		if transformer.Name == "" {
			v.addError(field, transformer.Name, "transformer name is required", "")
			continue
		}

		// Check for duplicate names
		if transformerNames[transformer.Name] {
			v.addError(field, transformer.Name, "duplicate transformer name", fmt.Sprintf("Transformer '%s' is defined multiple times", transformer.Name))
			continue
		}
		transformerNames[transformer.Name] = true

		// Validate transformer type
		if transformer.Type != "javascript" {
			v.addError(field+".type", transformer.Type, "unsupported transformer type", "Currently only 'javascript' is supported")
		}

		// Validate script is not empty
		if transformer.Script == "" {
			v.addError(field+".script", "", "transformer script is required", "")
		}
	}

	// Validate outputs
	for i, output := range cfg.Outputs {
		field := fmt.Sprintf("outputs[%d].%s", i, output.Name)

		if output.Name == "" {
			v.addError(field, output.Name, "output name is required", "")
			continue
		}

		// Check for duplicate names
		if outputNames[output.Name] {
			v.addError(field, output.Name, "duplicate output name", fmt.Sprintf("Output '%s' is defined multiple times", output.Name))
			continue
		}
		outputNames[output.Name] = true

		if output.Type == "modbus" {
			// Modbus output requires a target server name
			if output.Server == "" {
				v.addError(field+".server", output.Server, "modbus output requires 'server' to reference a hosted modbus server", "Set outputs[].server to the name of a servers[].name with type 'modbus'")
			}
		} else {
			// MQTT validation (existing)
			// Validate broker URL
			if err := validateBrokerURL(output.Broker, field+".broker"); err != nil {
				v.addError(field+".broker", output.Broker, err.Error(), "Expected format: 'protocol://host:port' or 'host:port'")
			}

			// Validate client ID
			if output.ClientID != "" {
				if err := validateClientID(output.ClientID); err != nil {
					v.addError(field+".client_id", output.ClientID, err.Error(), "Client ID must be alphanumeric with hyphens and underscores")
				}

				// Check for duplicate client IDs
				if clientIDs[output.ClientID] {
					v.addError(field+".client_id", output.ClientID, "duplicate client ID", fmt.Sprintf("Client ID '%s' is used by multiple components", output.ClientID))
				}
				clientIDs[output.ClientID] = true
			}

			// Validate topic
			if output.Topic == "" {
				v.addError(field+".topic", "", "output topic is required", "")
			} else {
				if err := validateTopic(output.Topic); err != nil {
					v.addError(field+".topic", output.Topic, err.Error(), "")
				}
			}

			// Validate QoS
			if output.QoS < 0 || output.QoS > 2 {
				v.addError(field+".qos", output.QoS, "QoS must be between 0 and 2", "Valid values: 0 (at most once), 1 (at least once), 2 (exactly once)")
			}
		}
	}

	// Validate pipelines reference valid inputs/outputs/transformers
	for _, pipeline := range cfg.Pipelines {
		if !inputNames[pipeline.Input] {
			v.addError(fmt.Sprintf("pipelines.%s.input", pipeline.Name), pipeline.Input, "references unknown input", fmt.Sprintf("Input '%s' is not defined", pipeline.Input))
		}

		for _, route := range pipeline.Routes {
			if !transformerNames[route.Transformer] {
				v.addError(fmt.Sprintf("pipelines.%s.transformer", pipeline.Name), route.Transformer, "references unknown transformer", fmt.Sprintf("Transformer '%s' is not defined", route.Transformer))
			}

			for _, output := range route.Outputs {
				if !outputNames[output] {
					v.addError(fmt.Sprintf("pipelines.%s.output", pipeline.Name), output, "references unknown output", fmt.Sprintf("Output '%s' is not defined", output))
				}
			}
		}
	}

	// If there are errors, return combined error
	if v.hasErrors() {
		var messages []string
		for _, err := range v.getErrors() {
			messages = append(messages, err.Error())
		}
		return fmt.Errorf("validation failed:\n%s", strings.Join(messages, "\n"))
	}

	return nil
}

// validateBrokerURL validates broker URL format
func validateBrokerURL(broker, field string) error {
	if broker == "" {
		return fmt.Errorf("broker URL is required")
	}

	// Check for invalid characters in broker URL
	if strings.Contains(broker, "@") {
		return fmt.Errorf("broker URL cannot contain @ character")
	}

	// Remove protocol prefix for parsing
	urlWithoutProtocol := broker
	if strings.HasPrefix(broker, "tls://") || strings.HasPrefix(broker, "ssl://") {
		urlWithoutProtocol = broker[6:] // Remove "tls://" or "ssl://"
	} else if strings.HasPrefix(broker, "ws://") {
		urlWithoutProtocol = broker[5:]
	} else if strings.HasPrefix(broker, "wss://") {
		urlWithoutProtocol = broker[6:]
	}

	// Try parsing as URL
	parsedURL, err := url.Parse("tcp://" + urlWithoutProtocol)
	if err != nil {
		return fmt.Errorf("invalid broker URL format")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("broker host is required")
	}

	// Validate host and port
	host, _, err := splitHostPort(parsedURL.Host)
	if err != nil {
		return fmt.Errorf("invalid host:port format: %v", err)
	}

	if host == "" {
		return fmt.Errorf("broker host cannot be empty")
	}

	// Validate hostname format - check for invalid characters
	// Valid hostnames can contain: alphanumeric, dots, hyphens, underscores, colons (for IPv6)
	hostnamePattern := regexp.MustCompile(`^[a-zA-Z0-9._:\-]+$`)
	if !hostnamePattern.MatchString(host) {
		return fmt.Errorf("invalid hostname format: %s (contains invalid characters)", host)
	}

	return nil
}

// validateClientID validates client ID format
func validateClientID(clientID string) error {
	// Client ID should be alphanumeric with hyphens, underscores, dots
	matched, err := regexp.MatchString(`^[a-zA-Z0-9\-_\.]+$`, clientID)
	if err != nil {
		return fmt.Errorf("error validating client ID: %v", err)
	}
	if !matched {
		return fmt.Errorf("invalid client ID format")
	}
	return nil
}

// validateTopic validates MQTT topic format
func validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	// Check for valid characters in topic
	matched, err := regexp.MatchString(`^[a-zA-Z0-9/+#_\-]+$`, topic)
	if err != nil {
		return fmt.Errorf("error validating topic: %v", err)
	}
	if !matched {
		return fmt.Errorf("topic contains invalid characters")
	}

	// Validate wildcard usage
	if strings.Contains(topic, "#") && strings.Index(topic, "#") != len(topic)-1 {
		return fmt.Errorf("multi-level wildcard (#) must be at the end of topic")
	}

	return nil
}

// splitHostPort splits host:port string
func splitHostPort(hostport string) (host, port string, err error) {
	parts := strings.Split(hostport, ":")
	if len(parts) == 1 {
		host = parts[0]
		port = ""
		return
	}
	if len(parts) == 2 {
		host = parts[0]
		port = parts[1]
		return
	}
	err = fmt.Errorf("invalid host:port format")
	return
}
