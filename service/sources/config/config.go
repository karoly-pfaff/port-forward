package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"portier/service/sources/domain"
	"portier/service/sources/validation"
)

// ErrMalformed marks a config whose bytes are not a valid Portier config
// container: invalid JSON, or a well-formed JSON value that is neither a rules
// array nor a {"rules": [...]} object. ErrSchemaInvalid marks a config that
// decodes as a rules container but contains at least one rule that fails
// validation. Callers (e.g. the recovery loader) classify load failures with
// errors.Is rather than matching message text.
var (
	ErrMalformed     = errors.New("config is malformed")
	ErrSchemaInvalid = errors.New("config has an invalid rule")
)

type Store struct {
	path string
}

func NewStore(path string) Store {
	return Store{path: path}
}

func (s Store) Load() ([]domain.ForwardRule, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.ForwardRule{}, nil
		}
		return nil, err
	}

	return Parse(raw)
}

// Parse decodes and validates raw config bytes into forward rules. It is the
// pure (no IO) core of Load, exposed so the startup recovery loader can classify
// a read-from-disk config without re-implementing the decode/validate rules.
// Failures wrap ErrMalformed (bad container) or ErrSchemaInvalid (bad rule).
func Parse(raw []byte) ([]domain.ForwardRule, error) {
	ruleItems, err := decodeRuleItems(raw)
	if err != nil {
		return nil, err
	}

	rules := make([]domain.ForwardRule, 0, len(ruleItems))
	for index, item := range ruleItems {
		rule, errs := validation.DecodeAndValidateForwardRule(item)
		if len(errs) > 0 {
			return nil, fmt.Errorf("%w: Invalid rule at index %d: %s", ErrSchemaInvalid, index, joinErrors(errs))
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func (s Store) Save(rules []domain.ForwardRule) error {
	for index, rule := range rules {
		validated, errors := validation.ValidateForwardRuleInput(validation.InputFromRule(rule))
		if len(errors) > 0 {
			return fmt.Errorf("Invalid rule at index %d: %s", index, joinErrors(errors))
		}
		rules[index] = validated
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(dir, ".portier-forwards-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, s.path); err != nil {
		if removeErr := os.Remove(s.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if retryErr := os.Rename(tempPath, s.path); retryErr != nil {
			return retryErr
		}
	}
	removeTemp = false
	return nil
}

func decodeRuleItems(raw []byte) ([]json.RawMessage, error) {
	var asArray []json.RawMessage
	if err := json.Unmarshal(raw, &asArray); err == nil {
		return asArray, nil
	}

	var asObject struct {
		Rules []json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(raw, &asObject); err == nil && asObject.Rules != nil {
		return asObject.Rules, nil
	}

	var syntaxError *json.SyntaxError
	if err := json.Unmarshal(raw, &json.RawMessage{}); errors.As(err, &syntaxError) {
		return nil, fmt.Errorf("%w: Invalid JSON: %v", ErrMalformed, err)
	}

	return nil, fmt.Errorf("%w: Config file must contain an array of forward rules.", ErrMalformed)
}

func joinErrors(errors []string) string {
	joined := ""
	for index, message := range errors {
		if index > 0 {
			joined += " "
		}
		joined += message
	}
	return joined
}
