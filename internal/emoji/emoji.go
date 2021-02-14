package emoji

// https://unicode.org/emoji/charts/full-emoji-list.html
const (
	HalfEclipse = "🌓"

	ThirdEclipse = "🌒"
	FullEclipse  = "🌑"
	EclipseFace  = "🌚"
	Comet        = "☄"

	FirstEclipse = "🌔"
	FullMoon     = "🌕"
	SunFace      = "🌞"
	Star         = "🌟"

	Zero = "🕸" //🥜
	Down = "🪱" //🌶 // 🥀
	Up   = "🦠" //🥦

	DotSnow  = "❄"
	DotFire  = "🔥"
	DotWater = "💧"
)

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
