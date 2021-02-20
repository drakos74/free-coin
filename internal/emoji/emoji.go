package emoji

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
)

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

// MapToSymbol maps the given number according to it's order.
func MapToSymbol(i string) string {
	symbol := HalfEclipse
	switch i {
	case "+3":
		symbol = FirstEclipse
	case "+2":
		symbol = FullMoon
	case "+1":
		symbol = SunFace
	case "+0":
		symbol = Star
	case "-3":
		symbol = ThirdEclipse
	case "-2":
		symbol = FullEclipse
	case "-1":
		symbol = EclipseFace
	case "-0":
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
