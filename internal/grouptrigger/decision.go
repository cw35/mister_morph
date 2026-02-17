package grouptrigger

import (
	"context"
	"strings"
	"time"
)

type Decision struct {
	Reason            string
	UsedAddressingLLM bool

	AddressingLLMAttempted bool
	AddressingLLMOK        bool
	Addressing             Addressing
}

type Addressing struct {
	Addressed      bool
	Confidence     float64
	WannaInterject bool
	Interject      float64
	Impulse        float64
	IsLightweight  bool
	Reason         string
}

type AddressingFunc func(ctx context.Context) (Addressing, bool, error)

type DecideOptions struct {
	Mode                     string
	ConfidenceThreshold      float64
	InterjectThreshold       float64
	ExplicitReason           string
	ExplicitMatched          bool
	AddressingFallbackReason string
	AddressingTimeout        time.Duration
	Addressing               AddressingFunc
}

func Decide(ctx context.Context, opts DecideOptions) (Decision, bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "smart"
	}

	confidenceThreshold := clamp01(opts.ConfidenceThreshold)

	interjectThreshold := clamp01(opts.InterjectThreshold)

	if opts.ExplicitMatched {
		return Decision{
			Reason: strings.TrimSpace(opts.ExplicitReason),
			Addressing: Addressing{
				Impulse: 1,
			},
		}, true, nil
	}

	if mode != "talkative" && mode != "smart" {
		return Decision{}, false, nil
	}

	dec := Decision{
		AddressingLLMAttempted: true,
		Reason:                 strings.TrimSpace(opts.AddressingFallbackReason),
	}
	if opts.Addressing == nil {
		return dec, false, nil
	}

	addrCtx := ctx
	if addrCtx == nil {
		addrCtx = context.Background()
	}
	cancel := func() {}
	if opts.AddressingTimeout > 0 {
		addrCtx, cancel = context.WithTimeout(addrCtx, opts.AddressingTimeout)
	}
	llmDec, llmOK, llmErr := opts.Addressing(addrCtx)
	cancel()
	if llmErr != nil {
		return dec, false, llmErr
	}
	llmDec = normalizeAddressing(llmDec)

	dec.AddressingLLMOK = llmOK
	dec.Addressing = llmDec
	if llmDec.Reason != "" {
		dec.Reason = llmDec.Reason
	}
	if !llmOK {
		return dec, false, nil
	}

	switch mode {
	case "smart":
		if llmDec.Addressed && llmDec.Confidence >= confidenceThreshold {
			dec.UsedAddressingLLM = true
			return dec, true, nil
		}
	case "talkative":
		if llmDec.WannaInterject && llmDec.Interject > interjectThreshold {
			dec.UsedAddressingLLM = true
			return dec, true, nil
		}
	}
	return dec, false, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func normalizeAddressing(in Addressing) Addressing {
	in.Confidence = clamp01(in.Confidence)
	in.Interject = clamp01(in.Interject)
	in.Impulse = clamp01(in.Impulse)
	in.Reason = strings.TrimSpace(in.Reason)
	return in
}
