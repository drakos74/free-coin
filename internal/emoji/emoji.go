package emoji

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
)

// https://unicode.org/emoji/charts/full-emoji-list.html
const (
	HalfEclipse = "🌓"

	ThirdEclipse = "🌒"
	FullEclipse  = "🌑"
	EclipseFace  = "🌚"
	Comet        = "🪐" // ☄

	FirstEclipse = "🌔"
	FullMoon     = "🌕"
	SunFace      = "🌞"
	Star         = "🌟"

	Zero = "🥜" //🕸
	Down = "🐞" //🌶 // 🥀
	Up   = "🦠" //🥦

	DotSnow  = "❄"
	DotFire  = "🔥"
	DotWater = "💧"

	Biohazard = "😝"
	Recycling = "🤑"

	Error = "🚫"

	HasValue = "🏳‍🌈"
	NoValue  = "‍☠️"

	Open  = "🔔"
	Close = "🔕"

	Money = "💰"
)

func MapBool(s bool) string {
	if s {
		return HasValue
	}
	return NoValue
}

func MapOpen(s bool) string {
	if s {
		return Open
	}
	return Close
}

// MapType maps the type of buy and sell to an emoji
func MapType(t model.Type) string {
	switch t {
	case model.Buy:
		return Recycling
	case model.Sell:
		return Biohazard
	}
	return Error
}

// MapToSign maps the given float value according to it's sign.
func MapToSign(f float64) string {
	emo := DotSnow
	if f > 0 {
		emo = DotFire
	} else if f < 0 {
		emo = DotWater
	}
	return emo
}

// MapToSentiment maps the given float value according to it's sign.
func MapToSentiment(f float64) string {
	emo := Zero
	if f > 0 {
		emo = Up
	} else if f < 0 {
		emo = Down
	}
	return emo
}

func MapToSymbols(ss []string) []string {
	emojiSlice := make([]string, len(ss))
	for j, s := range ss {
		emojiSlice[j] = MapToSymbol(s)
	}
	return emojiSlice
}

// MapValue maps the given value to an emoji
// it returns valuable results for values between [-5,5]
func MapValue(value float64) string {
	if value >= 4 {
		return Star
	} else if value >= 3 {
		return SunFace
	} else if value >= 2 {
		return FullMoon
	} else if value >= 1 {
		return FirstEclipse
	} else if value <= -4 {
		return Comet
	} else if value <= -3 {
		return EclipseFace
	} else if value <= -2 {
		return FullEclipse
	} else if value <= -1 {
		return ThirdEclipse
	}
	return HalfEclipse
}

// MapToSymbol maps the given number according to it's order.
func MapToSymbol(i string) string {
	symbol := HalfEclipse
	switch i {
	case "5":
		fallthrough
	case "6":
		symbol = FirstEclipse
	case "7":
		symbol = FullMoon
	case "8":
		symbol = SunFace
	case "9":
		fallthrough
	case "10":
		symbol = Star
	case "-5":
		fallthrough
	case "-6":
		symbol = ThirdEclipse
	case "-7":
		symbol = FullEclipse
	case "-8":
		symbol = EclipseFace
	case "-9":
		fallthrough
	case "-10":
		symbol = Comet
	}
	return symbol
}

var numberToTime = map[int]string{
	0:  "🕛", // twelve o'clock
	1:  "🕐", // one
	2:  "🕜", // one-thirty
	3:  "🕑", // two
	4:  "🕝", // two-thirty
	5:  "🕒", // three
	6:  "🕞", // three-thirty
	7:  "🕓", // four
	8:  "🕟", // four-thirty
	9:  "🕔", // five
	10: "🕠", // five-thirty
	//-0: "🕡", // six-thirty
	-1:  "🕚", // eleven
	-2:  "🕦", // eleven-thirty
	-3:  "🕙", // ten
	-4:  "🕥", // ten-thirty
	-5:  "🕘", // nine
	-6:  "🕤", // nine-thirty
	-7:  "🕗", // eight
	-8:  "🕣", // eight-thirty
	-9:  "🕖", // seven
	-10: "🕢", // seven-thirty
}

// MapNumber maps the given number according to it's order.
func MapNumber(i int) string {
	if s, ok := numberToTime[i]; ok {
		return s
	}
	if i < 0 {
		return "🕧" // twelve-thirty
	}
	return "🕕" // six o'clock
}

// PredictionList prints with emojis a numeric prediction list
func PredictionList(p buffer.PredictionList) string {
	// print only the 2-3 first predictions
	pp := make([]string, 2)
	for i, pr := range p {
		if i < 2 {
			pp[i] = Prediction(pr)
		}
	}
	return strings.Join(pp, " | ")
}

// Prediction prints with emojis a numeric Prediction
func Prediction(p *buffer.Prediction) string {
	return fmt.Sprintf("%s (%.2f | %.2f)", Sequence(p.Value), p.Probability, p.EMP)
}

// Sequence returns an emoji representation for the buffer sequence
func Sequence(s buffer.Sequence) string {
	ss := s.Values()
	symbols := MapToSymbols(ss)
	return strings.Join(symbols, " ")
}

// PredictionValues returns an emoji representation for a slice buffer sequence
func PredictionValues(ss []buffer.Sequence) string {
	symbols := make([]string, 0)
	for _, s := range ss {
		symbols = append(symbols, MapToSymbols(s.Values())...)
	}
	return strings.Join(symbols, "|")
}
