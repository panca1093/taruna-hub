package service

// EmojiAsset is one catalog emoji. ID is the lowercase hex codepoint and doubles as the Twemoji file name
// (web/public/emoji/<id>.svg), so every device draws the same picture.
type EmojiAsset struct {
	ID   string
	Name string
}

// EmojiAttribution is the Twemoji CC-BY 4.0 credit the app must show wherever the catalog is used.
const EmojiAttribution = "Emoji graphics: Twemoji by Twitter, Inc. and other contributors, licensed under CC-BY 4.0."

// emojiCatalog holds single-codepoint, visually distinct emoji only: no skin tones, flags or ZWJ sequences
// (FR-ADM-KEC-24). Every id has a bundled asset; a test keeps the two in step.
var emojiCatalog = []EmojiAsset{
	{"1f34e", "Apel"}, {"1f34c", "Pisang"}, {"1f347", "Anggur"}, {"1f349", "Semangka"}, {"1f353", "Stroberi"},
	{"1f34d", "Nanas"}, {"1f352", "Ceri"}, {"1f351", "Persik"}, {"1f34b", "Lemon"}, {"1f955", "Wortel"},
	{"1f33d", "Jagung"}, {"1f344", "Jamur"}, {"1f355", "Pizza"}, {"1f354", "Burger"}, {"1f36a", "Kue kering"},
	{"1f370", "Kue"}, {"1f436", "Anjing"}, {"1f431", "Kucing"}, {"1f42d", "Tikus"}, {"1f439", "Hamster"},
	{"1f430", "Kelinci"}, {"1f98a", "Rubah"}, {"1f43b", "Beruang"}, {"1f43c", "Panda"}, {"1f428", "Koala"},
	{"1f42f", "Harimau"}, {"1f981", "Singa"}, {"1f42e", "Sapi"}, {"1f437", "Babi"}, {"1f438", "Katak"},
	{"1f435", "Monyet"}, {"1f414", "Ayam"}, {"1f427", "Penguin"}, {"1f426", "Burung"}, {"1f422", "Kura-kura"},
	{"1f41f", "Ikan"}, {"1f419", "Gurita"}, {"1f98b", "Kupu-kupu"}, {"1f41d", "Lebah"}, {"1f40c", "Siput"},
	{"1f333", "Pohon"}, {"1f335", "Kaktus"}, {"1f33b", "Bunga matahari"}, {"1f339", "Mawar"}, {"1f340", "Semanggi"},
	{"1f308", "Pelangi"}, {"1f319", "Bulan sabit"}, {"1f525", "Api"}, {"1f4a7", "Tetesan air"}, {"2b50", "Bintang"},
	{"1f3b5", "Nada"}, {"1f514", "Lonceng"}, {"1f511", "Kunci"}, {"1f4a1", "Lampu"}, {"1f4d6", "Buku"},
	{"1f4cc", "Pin"}, {"1f3c6", "Piala"}, {"1f3a9", "Topi tinggi"}, {"1f451", "Mahkota"}, {"1f48e", "Permata"},
	{"1f380", "Pita"}, {"1f388", "Balon"}, {"1f381", "Hadiah"}, {"1f3b2", "Dadu"}, {"1f3af", "Target"},
	{"1f9e9", "Puzzle"}, {"1f697", "Mobil"}, {"1f68c", "Bus"}, {"1f682", "Kereta"}, {"1f680", "Roket"},
	{"26f5", "Perahu layar"}, {"1f6b2", "Sepeda"}, {"1f3e0", "Rumah"}, {"26bd", "Bola sepak"}, {"1f3c0", "Bola basket"},
	{"1f3be", "Tenis"}, {"1f3c8", "Bola rugbi"}, {"1f4f7", "Kamera"}, {"1f4bb", "Laptop"},
}

// ShapeAsset is one built-in flat shape; the web draws it from the id.
type ShapeAsset struct {
	ID   string
	Name string
}

var shapeCatalog = []ShapeAsset{
	{"kotak", "Kotak"}, {"segitiga", "Segitiga"}, {"persegi_panjang", "Persegi panjang"},
	{"lingkaran", "Lingkaran"}, {"jajar_genjang", "Jajar genjang"}, {"trapesium", "Trapesium"},
	{"belah_ketupat", "Belah ketupat"},
}

// symbolPalette is the click-to-add palette: arrows, stars, cards and geometry (FR-ADM-KEC-16b). It is
// text on purpose; none of these is an emoji, so the symbol rules accept all of them.
var symbolPalette = []string{
	"←", "↑", "→", "↓", "↔", "↕", "↖", "↗", "↘", "↙",
	"★", "☆", "♠", "♣", "♥", "♦", "♡", "♢",
	"●", "○", "■", "□", "▲", "△", "▼", "▽", "◆", "◇", "◐", "◑",
	"+", "×", "÷", "=", "≠", "≈", "≤", "≥", "∞", "∑", "∆", "π", "Ω", "α", "β", "γ",
}

func EmojiCatalog() []EmojiAsset { return append([]EmojiAsset(nil), emojiCatalog...) }
func ShapeCatalog() []ShapeAsset { return append([]ShapeAsset(nil), shapeCatalog...) }
func SymbolPalette() []string    { return append([]string(nil), symbolPalette...) }

func emojiName(id string) (string, bool) {
	for _, e := range emojiCatalog {
		if e.ID == id {
			return e.Name, true
		}
	}
	return "", false
}

func shapeName(id string) (string, bool) {
	for _, s := range shapeCatalog {
		if s.ID == id {
			return s.Name, true
		}
	}
	return "", false
}
