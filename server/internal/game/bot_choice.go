package game

import "math"

// Weighted choices for bot decisions.

type Option[T any] struct {
	Value T
	Score float64
}

// WeightedPick chooses "like a person": softmax over scores normalised to [0, 1]. The weaker the bot, the
// higher the temperature and the more often it takes something other than the best option.
func WeightedPick[T any](rng func() float64, options []Option[T], skill float64) T {
	if len(options) == 0 {
		panic("weightedPick: no options")
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, o := range options {
		lo, hi = math.Min(lo, o.Score), math.Max(hi, o.Score)
	}
	span := hi - lo
	if span == 0 {
		span = 1
	}
	t := BotConfig.Temperature.Min + (1-skill)*(BotConfig.Temperature.Max-BotConfig.Temperature.Min)
	weights := make([]float64, len(options))
	total := 0.0
	for i, o := range options {
		weights[i] = math.Exp(((o.Score-lo)/span - 1) / t)
		total += weights[i]
	}
	r := rng() * total
	for i, o := range options {
		r -= weights[i]
		if r <= 0 {
			return o.Value
		}
	}
	return options[len(options)-1].Value
}

// Follows: a strong bot keeps the rule with probability Strong, a weak one Weak, linear in between.
func Follows(rng func() float64, skill float64, rule StrongWeak) bool {
	return rng() < rule.Weak+(rule.Strong-rule.Weak)*skill
}
