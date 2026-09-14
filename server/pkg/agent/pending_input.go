package agent

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	PendingInputVersion1              = 1
	PendingInputMaxBodyBytes          = 32 * 1024
	PendingInputMaxQuestions          = 3
	PendingInputMaxOptions            = 8
	PendingInputMaxQuestionRunes      = 2000
	PendingInputMaxHeaderRunes        = 120
	PendingInputMaxLabelRunes         = 200
	PendingInputMaxDescriptionRunes   = 500
	PendingInputMaxAnswersPerQuestion = 8
	PendingInputMaxAnswerRunes        = 2000
)

var ErrPendingInputUnsupported = errors.New("provider user input request is unsupported")

type PendingInputRequest struct {
	Version    int                    `json:"version"`
	RequestKey string                 `json:"request_key"`
	Blocking   bool                   `json:"blocking"`
	Questions  []PendingInputQuestion `json:"questions"`
}

type PendingInputQuestion struct {
	ID          string               `json:"id"`
	Header      string               `json:"header"`
	Question    string               `json:"question"`
	Options     []PendingInputOption `json:"options"`
	AllowOther  bool                 `json:"allow_other"`
	MultiSelect bool                 `json:"multi_select"`
}

type PendingInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type PendingInputAnswer struct {
	Answers     map[string][]string         `json:"answers"`
	OnDelivered func(context.Context) error `json:"-"`
}

func (a PendingInputAnswer) MarkDelivered(ctx context.Context) error {
	if a.OnDelivered == nil {
		return nil
	}
	// Native delivery already happened; normal process cleanup must not cancel
	// its receipt. Keep the acknowledgement independently bounded.
	deliveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return a.OnDelivered(deliveryCtx)
}

// NewPendingInputRequestKey creates an opaque, stable correlation key from
// provider-native request identities without retaining those identities in the
// server-visible key.
func NewPendingInputRequestKey(parts ...string) string {
	h := sha256.New()
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func ValidatePendingInputRequest(req PendingInputRequest) error {
	if req.Version != PendingInputVersion1 {
		return fmt.Errorf("pending input version %d is unsupported", req.Version)
	}
	if !req.Blocking {
		return fmt.Errorf("%w: nonblocking requests are not supported", ErrPendingInputUnsupported)
	}
	if !strings.HasPrefix(req.RequestKey, "sha256:") || len(req.RequestKey) != len("sha256:")+sha256.Size*2 {
		return errors.New("pending input request_key is invalid")
	}
	if len(req.Questions) == 0 || len(req.Questions) > PendingInputMaxQuestions {
		return fmt.Errorf("pending input must contain 1-%d questions", PendingInputMaxQuestions)
	}
	seen := make(map[string]struct{}, len(req.Questions))
	for _, question := range req.Questions {
		if question.ID == "" || !pendingInputStringIsCanonical(question.ID) || utf8.RuneCountInString(question.ID) > PendingInputMaxLabelRunes {
			return errors.New("pending input question id is invalid")
		}
		if _, exists := seen[question.ID]; exists {
			return errors.New("pending input question ids must be unique")
		}
		seen[question.ID] = struct{}{}
		if !pendingInputStringIsCanonical(question.Header) || utf8.RuneCountInString(question.Header) > PendingInputMaxHeaderRunes {
			return errors.New("pending input question header is invalid")
		}
		if question.Question == "" || !pendingInputStringIsCanonical(question.Question) || utf8.RuneCountInString(question.Question) > PendingInputMaxQuestionRunes {
			return errors.New("pending input question text is invalid")
		}
		if len(question.Options) > PendingInputMaxOptions {
			return fmt.Errorf("pending input question has more than %d options", PendingInputMaxOptions)
		}
		if len(question.Options) == 0 && !question.AllowOther {
			return errors.New("pending input question requires an option or allow_other")
		}
		seenLabels := make(map[string]struct{}, len(question.Options))
		for _, option := range question.Options {
			if option.Label == "" || !pendingInputStringIsCanonical(option.Label) || !pendingInputStringIsCanonical(option.Description) || utf8.RuneCountInString(option.Label) > PendingInputMaxLabelRunes || utf8.RuneCountInString(option.Description) > PendingInputMaxDescriptionRunes {
				return errors.New("pending input option is invalid")
			}
			if _, duplicate := seenLabels[option.Label]; duplicate {
				return errors.New("pending input option labels must be unique")
			}
			seenLabels[option.Label] = struct{}{}
		}
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal pending input request: %w", err)
	}
	if len(encoded) > PendingInputMaxBodyBytes {
		return fmt.Errorf("pending input request exceeds %d bytes", PendingInputMaxBodyBytes)
	}
	return nil
}

func ValidatePendingInputAnswer(req PendingInputRequest, answer PendingInputAnswer) error {
	known := make(map[string]PendingInputQuestion, len(req.Questions))
	for _, question := range req.Questions {
		known[question.ID] = question
	}
	if len(answer.Answers) != len(req.Questions) {
		return errors.New("pending input answer must contain every question")
	}
	for id, values := range answer.Answers {
		question, ok := known[id]
		if !ok {
			return fmt.Errorf("pending input answer contains unknown question %q", id)
		}
		if len(values) == 0 || len(values) > PendingInputMaxAnswersPerQuestion || (!question.MultiSelect && len(values) != 1) {
			return fmt.Errorf("pending input answer count is invalid for question %q", id)
		}
		seenValues := make(map[string]struct{}, len(values))
		for _, value := range values {
			if value == "" || !pendingInputStringIsCanonical(value) || utf8.RuneCountInString(value) > PendingInputMaxAnswerRunes {
				return fmt.Errorf("pending input answer is invalid for question %q", id)
			}
			if _, duplicate := seenValues[value]; duplicate {
				return fmt.Errorf("pending input answer is duplicated for question %q", id)
			}
			seenValues[value] = struct{}{}
			if len(question.Options) > 0 && !question.AllowOther {
				matched := false
				for _, option := range question.Options {
					if value == option.Label {
						matched = true
						break
					}
				}
				if !matched {
					return fmt.Errorf("pending input answer is not an allowed option for question %q", id)
				}
			}
		}
	}
	wireAnswers := make(map[string]struct {
		Answers []string `json:"answers"`
	}, len(answer.Answers))
	for id, values := range answer.Answers {
		wireAnswers[id] = struct {
			Answers []string `json:"answers"`
		}{Answers: values}
	}
	encoded, err := json.Marshal(struct {
		Answers map[string]struct {
			Answers []string `json:"answers"`
		} `json:"answers"`
	}{Answers: wireAnswers})
	if err != nil {
		return fmt.Errorf("marshal pending input answer: %w", err)
	}
	if len(encoded) > PendingInputMaxBodyBytes {
		return fmt.Errorf("pending input answer exceeds %d bytes", PendingInputMaxBodyBytes)
	}
	return nil
}

func pendingInputStringIsCanonical(value string) bool {
	return strings.TrimSpace(value) == value && !strings.ContainsRune(value, '\x00')
}
