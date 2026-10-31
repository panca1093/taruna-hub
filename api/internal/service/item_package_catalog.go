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
	{"1f350", "Pir"}, {"1f34a", "Jeruk"}, {"1f348", "Melon"}, {"1f96d", "Mangga"}, {"1f95d", "Kiwi"},
	{"1f345", "Tomat"}, {"1f965", "Kelapa"}, {"1f951", "Alpukat"}, {"1f346", "Terong"}, {"1f954", "Kentang"},
	{"1f952", "Mentimun"}, {"1f966", "Brokoli"}, {"1f9c4", "Bawang putih"}, {"1f9c5", "Bawang bombai"}, {"1f95c", "Kacang"},
	{"1f330", "Kastanye"}, {"1f35e", "Roti"}, {"1f950", "Croissant"}, {"1f956", "Baguette"}, {"1f968", "Pretzel"},
	{"1f9c0", "Keju"}, {"1f95a", "Telur"}, {"1f373", "Telur ceplok"}, {"1f95e", "Panekuk"}, {"1f9c7", "Wafel"},
	{"1f357", "Paha ayam"}, {"1f356", "Daging"}, {"1f32d", "Hot dog"}, {"1f35f", "Kentang goreng"}, {"1f96a", "Roti lapis"},
	{"1f32e", "Taco"}, {"1f32f", "Burrito"}, {"1f957", "Salad"}, {"1f37f", "Popcorn"}, {"1f35a", "Nasi"},
	{"1f359", "Nasi kepal"}, {"1f35c", "Mi kuah"}, {"1f35d", "Spageti"}, {"1f363", "Sushi"}, {"1f364", "Udang goreng"},
	{"1f95f", "Pangsit"}, {"1f366", "Es krim"}, {"1f369", "Donat"}, {"1f382", "Kue ulang tahun"}, {"1f9c1", "Cupcake"},
	{"1f36b", "Cokelat"}, {"1f36c", "Permen"}, {"1f36d", "Lolipop"}, {"1f36f", "Madu"}, {"1f37c", "Botol susu"},
	{"2615", "Kopi"}, {"1f375", "Teh"}, {"1f9c3", "Kotak jus"}, {"1f964", "Gelas minum"}, {"1f9ca", "Es batu"},
	{"1f95b", "Susu"}, {"1f434", "Kuda"}, {"1f984", "Unicorn"}, {"1f993", "Zebra"}, {"1f992", "Jerapah"},
	{"1f418", "Gajah"}, {"1f98f", "Badak"}, {"1f99b", "Kuda nil"}, {"1f42a", "Unta"}, {"1f998", "Kanguru"},
	{"1f410", "Kambing"}, {"1f411", "Domba"}, {"1f98c", "Rusa"}, {"1f994", "Landak"}, {"1f987", "Kelelawar"},
	{"1f43a", "Serigala"}, {"1f99d", "Rakun"}, {"1f9a8", "Sigung"}, {"1f9a6", "Berang-berang"}, {"1f9a5", "Kungkang"},
	{"1f986", "Bebek"}, {"1f985", "Elang"}, {"1f989", "Burung hantu"}, {"1f99a", "Merak"},
	{"1f99c", "Burung beo"}, {"1f9a9", "Flamingo"}, {"1f40a", "Buaya"}, {"1f40d", "Ular"}, {"1f98e", "Kadal"},
	{"1f996", "T-rex"}, {"1f995", "Dinosaurus"}, {"1f433", "Paus"}, {"1f42c", "Lumba-lumba"}, {"1f988", "Hiu"},
	{"1f420", "Ikan tropis"}, {"1f421", "Ikan buntal"}, {"1f980", "Kepiting"}, {"1f99e", "Lobster"}, {"1f990", "Udang"},
	{"1f991", "Cumi-cumi"}, {"1f41a", "Kerang"}, {"1f41e", "Kepik"}, {"1f41c", "Semut"}, {"1f997", "Jangkrik"},
	{"1f982", "Kalajengking"}, {"1f41b", "Ulat"}, {"1fab2", "Kumbang"}, {"1f98d", "Gorila"}, {"1f999", "Llama"},
	{"1f332", "Cemara"}, {"1f334", "Pohon kelapa"}, {"1f331", "Tunas"}, {"1f33f", "Daun herba"}, {"1f341", "Daun maple"},
	{"1f337", "Tulip"}, {"1f338", "Bunga sakura"}, {"1f33a", "Kembang sepatu"}, {"1f33c", "Bunga kuning"},
	{"1f490", "Buket"}, {"1f33e", "Padi"}, {"1f30d", "Bumi"}, {"1f31e", "Matahari"},
	{"26c4", "Manusia salju"}, {"26a1", "Petir"}, {"1f30a", "Ombak"}, {"1f30b", "Gunung api"}, {"1f5fb", "Gunung"},
	{"1faa8", "Batu"}, {"1fab5", "Kayu"}, {"231a", "Jam tangan"}, {"23f0", "Jam weker"},
	{"231b", "Jam pasir"}, {"1f4f1", "Ponsel"}, {"1f4be", "Disket"}, {"1f4bf", "Cakram"}, {"1f4fa", "Televisi"},
	{"1f4fb", "Radio"}, {"1f526", "Senter"}, {"1f50b", "Baterai"}, {"1f50c", "Steker"}, {"1f4b0", "Kantong uang"},
	{"1f4b3", "Kartu"}, {"1f4e6", "Kotak paket"}, {"1f4cf", "Penggaris"}, {"1f4d0", "Penggaris segitiga"}, {"1f4ce", "Penjepit kertas"},
	{"1f512", "Gembok"}, {"1f528", "Palu"}, {"1fa93", "Kapak"}, {"1f527", "Kunci pas"}, {"1f529", "Mur baut"},
	{"1f9f2", "Magnet"}, {"1f9ea", "Tabung reaksi"}, {"1f52c", "Mikroskop"}, {"1f52d", "Teleskop"}, {"1f48a", "Pil"},
	{"1fa79", "Plester"}, {"1f9f8", "Boneka beruang"}, {"1fa81", "Layang-layang"}, {"1fa80", "Yoyo"}, {"1f389", "Terompet pesta"},
	{"1f9e8", "Petasan"},
	{"1f3a8", "Palet"}, {"1f9f5", "Benang"}, {"1f9f6", "Gulungan benang"}, {"1f453", "Kacamata"}, {"1f455", "Kaus"},
	{"1f456", "Celana"}, {"1f457", "Gaun"}, {"1f45f", "Sepatu"}, {"1f460", "Sepatu hak"}, {"1f462", "Sepatu bot"},
	{"1f9e2", "Topi"}, {"1f392", "Ransel"}, {"1f45c", "Tas tangan"}, {"1f48d", "Cincin"}, {"1f302", "Payung"},
	{"1f9e4", "Sarung tangan"}, {"1f9e3", "Syal"}, {"1f9e6", "Kaus kaki"}, {"1fa91", "Kursi"}, {"1f6aa", "Pintu"},
	{"1fa9f", "Jendela"}, {"1f6c1", "Bak mandi"}, {"1f9f9", "Sapu"}, {"1f9fa", "Keranjang"}, {"1f9fb", "Tisu gulung"},
	{"1f9fc", "Sabun"}, {"1faa3", "Ember"}, {"1f3ee", "Lampion"}, {"1f4da", "Tumpukan buku"}, {"1f4d3", "Buku catatan"},
	{"1f4dc", "Gulungan"}, {"1f4f0", "Koran"}, {"1f4e3", "Megafon"}, {"1f3fa", "Guci"}, {"2693", "Jangkar"},
	{"1f9ed", "Kompas"}, {"26fa", "Tenda"}, {"1f3aa", "Sirkus"}, {"1f5ff", "Patung"}, {"1f3b8", "Gitar"},
	{"1f3b9", "Piano"}, {"1f3ba", "Trompet"}, {"1f3bb", "Biola"}, {"1f941", "Drum"}, {"1f3b7", "Saksofon"},
	{"1f3a4", "Mikrofon"}, {"1f3a7", "Headphone"}, {"1f3ae", "Konsol game"}, {"1f0cf", "Kartu joker"}, {"1f3b3", "Boling"},
	{"1f3d3", "Pingpong"}, {"1f3f8", "Bulu tangkis"}, {"1f94a", "Sarung tinju"}, {"1f3d0", "Bola voli"}, {"26be", "Bisbol"},
	{"1f3cf", "Kriket"}, {"26f3", "Golf"}, {"1f3a3", "Pancing"}, {"1f6f9", "Papan seluncur"}, {"1f6fc", "Sepatu roda"},
	{"1f947", "Medali emas"}, {"1f3ab", "Tiket"}, {"1fa83", "Bumerang"}, {"1f3b1", "Bola biliar"}, {"1f6f7", "Kereta luncur"},
	{"1f3bf", "Ski"}, {"1f3f9", "Busur panah"}, {"1f695", "Taksi"}, {"1f693", "Mobil polisi"}, {"1f691", "Ambulans"},
	{"1f692", "Mobil pemadam"}, {"1f69a", "Truk"}, {"1f69c", "Traktor"}, {"1f6f5", "Skuter"}, {"1f681", "Helikopter"},
	{"1f6f8", "UFO"}, {"1f6a4", "Speedboat"}, {"1f6a2", "Kapal"}, {"1f6f6", "Kano"}, {"1f6f4", "Otoped"},
	{"1f6a6", "Lampu lalu lintas"}, {"1f6a7", "Rambu konstruksi"}, {"26fd", "Pom bensin"}, {"1f3f0", "Kastil"}, {"1f5fc", "Menara"},
	{"1f3ed", "Pabrik"}, {"1f3e5", "Rumah sakit"}, {"1f3eb", "Sekolah"}, {"1f3a1", "Bianglala"}, {"1f3a2", "Roller coaster"},
	{"1f3a0", "Komidi putar"}, {"26f2", "Air mancur"}, {"1f309", "Jembatan"}, {"1f3e1", "Rumah taman"},
	{"1f4a5", "Ledakan"},
}

// ShapeAsset is one built-in shape; the web draws it from the id. Group is "flat" (bangun datar) or "solid"
// (bangun ruang, drawn as a shaded silhouette).
type ShapeAsset struct {
	ID    string
	Name  string
	Group string
}

const (
	ShapeFlat  = "flat"
	ShapeSolid = "solid"
)

var shapeCatalog = []ShapeAsset{
	{"kotak", "Kotak", ShapeFlat}, {"segitiga", "Segitiga", ShapeFlat}, {"persegi_panjang", "Persegi panjang", ShapeFlat},
	{"lingkaran", "Lingkaran", ShapeFlat}, {"jajar_genjang", "Jajar genjang", ShapeFlat}, {"trapesium", "Trapesium", ShapeFlat},
	{"belah_ketupat", "Belah ketupat", ShapeFlat},
	{"segi_lima", "Segi lima", ShapeFlat}, {"segi_enam", "Segi enam", ShapeFlat}, {"segi_delapan", "Segi delapan", ShapeFlat},
	{"bintang", "Bintang", ShapeFlat}, {"oval", "Oval", ShapeFlat}, {"setengah_lingkaran", "Setengah lingkaran", ShapeFlat},
	{"layang_layang", "Layang-layang", ShapeFlat}, {"tanda_tambah", "Tanda tambah", ShapeFlat}, {"hati", "Hati", ShapeFlat},
	{"panah", "Panah", ShapeFlat},
	{"kubus", "Kubus", ShapeSolid}, {"balok", "Balok", ShapeSolid}, {"tabung", "Tabung", ShapeSolid},
	{"kerucut", "Kerucut", ShapeSolid}, {"bola", "Bola", ShapeSolid}, {"limas", "Limas", ShapeSolid},
	{"prisma", "Prisma", ShapeSolid},
}

// defaultShapeCount is how many leading catalog shapes a test without a shape package draws from. It stays at the
// original seven: tests resolve that pool at every admission, so growing it would change tests already locked.
const defaultShapeCount = 7

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
