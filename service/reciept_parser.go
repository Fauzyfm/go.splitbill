package service

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gosplitbill/model"

	"github.com/google/uuid"
)

// =====================================================================
// Bentuk data
// =====================================================================

// Respons dari API OCR (FastAPI / PaddleOCR).
type OCRLine struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Box   [4]int  `json:"box"` // x1, y1, x2, y2
}

type OCRResponse struct {
	Success bool      `json:"success"`
	Text    string    `json:"text"`
	Lines   []OCRLine `json:"lines"`
}



// =====================================================================
// Parsing angka (Rupiah)
// =====================================================================

var (
	// Dengan pemisah ribuan: 17,000 | 17.000 | Rp 17.000 | 17.000,00 | -5.000 | (5.000) | 17.000,-
	strictAmountRe = regexp.MustCompile(`^(\(|-)?(?:(?i:rp)\.?\s*)?(\d{1,3}(?:[.,\s]\d{3})+)(?:[.,]\d{1,2})?(,-|\)|-)?$`)
	// Tanpa pemisah: 15000 (3-9 digit). Hanya dipercaya jika sejajar dengan kolom harga.
	plainAmountRe = regexp.MustCompile(`^(\(|-)?(?:(?i:rp)\.?\s*)?(\d{3,9})(,-|\)|-)?$`)

	smallIntRe    = regexp.MustCompile(`^[xX]?(\d{1,2})[xX]?$`)
	leadingQtyRe  = regexp.MustCompile(`^(\d{1,2})\s*[xX]?\s+(\S.{2,})$`)
	detailRe      = regexp.MustCompile(`(\d{1,3})\s*[xX*]\s*@?\s*(\S+)`)
	detailNoiseRe = regexp.MustCompile(`(?i)rp|pcs|pc|[xX@*=]`)
)

type amount struct {
	value  int64
	neg    bool
	strict bool // true jika ada pemisah ribuan / awalan Rp
}

// OCR sering menukar angka dengan huruf mirip. Hanya dipakai bila teks memuat minimal satu digit.
func normalizeDigits(s string) string {
	hasDigit := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return s
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case 'O', 'o':
			return '0'
		case 'l', 'I', '|':
			return '1'
		}
		return r
	}, s)
}

func digitsToInt(s string) int64 {
	var n int64
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int64(r-'0')
		}
	}
	return n
}

func isNeg(prefix, suffix string) bool { return prefix == "-" || prefix == "(" || suffix == "-" }

func parseAmount(raw string) (amount, bool) {
	s := normalizeDigits(strings.TrimSpace(raw))
	if m := strictAmountRe.FindStringSubmatch(s); m != nil {
		return amount{value: digitsToInt(m[2]), neg: isNeg(m[1], m[3]), strict: true}, true
	}
	if m := plainAmountRe.FindStringSubmatch(s); m != nil {
		return amount{value: digitsToInt(m[2]), neg: isNeg(m[1], m[3])}, true
	}
	return amount{}, false
}

// =====================================================================
// Pencocokan label (toleran salah baca OCR)
// =====================================================================

type rowKind int

const (
	kindNone rowKind = iota
	kindSubtotal
	kindTax
	kindService
	kindDiscount
	kindDiscountWeak // "promo"/"voucher": bisa nama item, hanya dipercaya jika nilainya negatif / di zona ringkasan
	kindPayment
	kindTotal
)

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// tokenIs: awalan sama ("servicecharge" ~ "service") atau beda maksimal 1 huruf untuk kata >= 5 huruf ("subtota1" ~ "subtotal").
func tokenIs(tok, key string) bool {
	if strings.HasPrefix(tok, key) {
		return true
	}
	return len(key) >= 5 && levenshtein(tok, key) <= 1
}

func classify(label string) rowKind {
	toks := tokenize(label)

	anyTok := func(keys ...string) bool {
		for _, t := range toks {
			for _, k := range keys {
				if tokenIs(t, k) {
					return true
				}
			}
		}
		return false
	}
	eqTok := func(keys ...string) bool { // kata persis (agar "cashew" tidak dianggap "cash")
		for _, t := range toks {
			for _, k := range keys {
				if t == k {
					return true
				}
			}
		}
		return false
	}

	switch {
	case anyTok("subtotal") || (eqTok("sub") && anyTok("total")):
		return kindSubtotal
	case anyTok("tax", "pajak", "ppn", "pb1"):
		return kindTax
	case anyTok("service", "servis", "svc", "layanan"):
		return kindService
	case anyTok("diskon", "discount", "disc", "potongan"):
		return kindDiscount
	case anyTok("promo", "voucher"):
		return kindDiscountWeak
	case eqTok("tunai", "cash", "kembali", "kembalian", "change", "debit", "kredit", "credit",
		"qris", "gopay", "ovo", "dana", "shopeepay", "linkaja", "edc"):
		return kindPayment
	case anyTok("total"):
		return kindTotal
	}
	return kindNone
}

func hasWordLetters(s string) bool {
	for _, r := range detailNoiseRe.ReplaceAllString(s, "") {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// =====================================================================
// Langkah 1: gabungkan teks yang sebaris
// =====================================================================

type receiptRow struct {
	cells []OCRLine // terurut kiri -> kanan
	midY  float64
	h     int // tinggi teks terbesar di baris
}

func (l OCRLine) midY() float64 { return float64(l.Box[1]+l.Box[3]) / 2 }
func (l OCRLine) height() int   { return l.Box[3] - l.Box[1] }

func (r receiptRow) text() string {
	parts := make([]string, 0, len(r.cells))
	for _, c := range r.cells {
		parts = append(parts, strings.TrimSpace(c.Text))
	}
	return strings.Join(parts, " ")
}

func (r receiptRow) cellsExcept(idx int) []OCRLine {
	out := make([]OCRLine, 0, len(r.cells))
	for i, c := range r.cells {
		if i != idx {
			out = append(out, c)
		}
	}
	return out
}

func (r receiptRow) label(priceIdx int) string {
	parts := make([]string, 0, len(r.cells))
	for _, c := range r.cellsExcept(priceIdx) {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, " ")
}

func medianHeight(lines []OCRLine) int {
	if len(lines) == 0 {
		return 0
	}
	hs := make([]int, 0, len(lines))
	for _, l := range lines {
		hs = append(hs, l.height())
	}
	sort.Ints(hs)
	return hs[len(hs)/2]
}

func groupRows(lines []OCRLine) []receiptRow {
	if len(lines) == 0 {
		return nil
	}
	tol := float64(medianHeight(lines)) * 0.6

	sorted := append([]OCRLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].midY() < sorted[j].midY() })

	var rows []receiptRow
	for _, l := range sorted {
		n := len(rows)
		if n > 0 && math.Abs(l.midY()-rows[n-1].midY) <= tol {
			r := &rows[n-1]
			r.cells = append(r.cells, l)
			cnt := float64(len(r.cells))
			r.midY = (r.midY*(cnt-1) + l.midY()) / cnt
		} else {
			rows = append(rows, receiptRow{cells: []OCRLine{l}, midY: l.midY()})
		}
	}
	for i := range rows {
		cells := rows[i].cells
		sort.Slice(cells, func(a, b int) bool { return cells[a].Box[0] < cells[b].Box[0] })
		for _, c := range cells {
			if c.height() > rows[i].h {
				rows[i].h = c.height()
			}
		}
	}
	return rows
}

// =====================================================================
// Langkah 2: deteksi kolom harga (rata kanan)
// =====================================================================

type priceColumn struct {
	x, tol float64
	ok     bool
}

// Semua angka harga di sebuah struk hampir selalu punya tepi kanan (x2) yang sama.
// Kelompok x2 terbesar = kolom harga. Jika imbang, pilih yang paling kanan.
func detectPriceColumn(lines []OCRLine) priceColumn {
	pageW := 0
	for _, l := range lines {
		if l.Box[2] > pageW {
			pageW = l.Box[2]
		}
	}
	tol := math.Max(12, float64(pageW)*0.03)

	var xs []float64
	for _, l := range lines {
		a, ok := parseAmount(l.Text)
		if !ok {
			continue
		}
		if a.strict || len(strconv.FormatInt(a.value, 10)) >= 4 {
			xs = append(xs, float64(l.Box[2]))
		}
	}
	if len(xs) == 0 {
		return priceColumn{}
	}
	sort.Float64s(xs)

	bestStart, bestEnd, start := 0, 0, 0
	for i := 1; i <= len(xs); i++ {
		if i == len(xs) || xs[i]-xs[i-1] > tol {
			if i-start >= bestEnd-bestStart {
				bestStart, bestEnd = start, i
			}
			start = i
		}
	}
	sum := 0.0
	for _, x := range xs[bestStart:bestEnd] {
		sum += x
	}
	return priceColumn{x: sum / float64(bestEnd-bestStart), tol: tol, ok: true}
}

// priceIndex mencari sel harga pada baris: yang sejajar kolom harga (paling kanan diutamakan);
// cadangan: sel paling kanan jika formatnya jelas harga (ada pemisah ribuan).
func (r receiptRow) priceIndex(col priceColumn) (int, amount, bool) {
	if col.ok {
		for i := len(r.cells) - 1; i >= 0; i-- {
			c := r.cells[i]
			if math.Abs(float64(c.Box[2])-col.x) > col.tol {
				continue
			}
			if a, ok := parseAmount(c.Text); ok {
				return i, a, true
			}
		}
	}
	last := len(r.cells) - 1
	if a, ok := parseAmount(r.cells[last].Text); ok && a.strict {
		return last, a, true
	}
	return -1, amount{}, false
}

// =====================================================================
// Langkah 3: nama toko
// =====================================================================

// Nama/logo toko biasanya dicetak lebih besar. Ambil baris teratas yang tingginya >= 1.25x median.
// Cadangan: baris pertama yang mengandung huruf.
func detectTitle(rows []receiptRow, medianH int) string {
	var parts []string
	for _, r := range rows {
		if float64(r.h) < float64(medianH)*1.25 {
			break
		}
		parts = append(parts, r.text())
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	for _, r := range rows {
		if countLetters(r.text()) >= 3 {
			return r.text()
		}
	}
	return ""
}

// =====================================================================
// Langkah 4: baca baris item
// =====================================================================

type itemDraft struct {
	name    string
	qty     int
	printed int64 // angka di kolom harga (bisa total per baris, bisa harga satuan; diputuskan di akhir)
	unit    int64 // harga satuan dari baris rincian "2 x 12,000" (0 jika tidak ada)
}

// Baris item yang punya nama: "[qty] nama [harga satuan]  total"
func parseItemCells(cells []OCRLine) (name string, qty int, unit int64) {
	qty = 1
	qtySet := false
	var names []string
	for _, c := range cells {
		t := strings.TrimSpace(c.Text)
		if hasWordLetters(t) {
			names = append(names, t)
			continue
		}
		if m := smallIntRe.FindStringSubmatch(t); m != nil && !qtySet {
			if q, _ := strconv.Atoi(m[1]); q > 0 {
				qty = q
			}
			qtySet = true
			continue
		}
		if a, ok := parseAmount(t); ok && unit == 0 {
			unit = a.value
		}
	}
	name = strings.Join(names, " ")
	if !qtySet { // "2 Es Teh" yang menyatu dalam satu sel
		if m := leadingQtyRe.FindStringSubmatch(name); m != nil {
			if q, _ := strconv.Atoi(m[1]); q > 0 {
				qty = q
				name = m[2]
			}
		}
	}
	return name, qty, unit
}

// Baris rincian tanpa nama: "2 x 12,000" (nama ada di baris atasnya)
func parseDetail(cells []OCRLine) (qty int, unit int64) {
	qty = 1
	parts := make([]string, 0, len(cells))
	for _, c := range cells {
		parts = append(parts, strings.TrimSpace(c.Text))
	}
	if m := detailRe.FindStringSubmatch(strings.Join(parts, " ")); m != nil {
		if q, _ := strconv.Atoi(m[1]); q > 0 {
			qty = q
		}
		if a, ok := parseAmount(m[2]); ok {
			unit = a.value
		}
		return qty, unit
	}
	qtySet := false
	for _, t := range parts { // sel terpisah: "2" | "12,000"
		if m := smallIntRe.FindStringSubmatch(t); m != nil && !qtySet {
			if q, _ := strconv.Atoi(m[1]); q > 0 {
				qty = q
			}
			qtySet = true
			continue
		}
		if a, ok := parseAmount(t); ok && unit == 0 {
			unit = a.value
		}
	}
	return qty, unit
}

// =====================================================================
// Langkah 5: ekstraksi utama
// =====================================================================

func ParseReceipt(resp OCRResponse, userid uuid.UUID) model.ParsedReceipt {
	out := model.ParsedReceipt{Items: []model.BillItem{}, Warnings: []string{}}
	out.Bill.Status = model.StatusBillDraft
	out.Bill.OcrRawText = resp.Text
	out.Bill.UserID = userid


	warn := func(format string, args ...any) {
		out.Warnings = append(out.Warnings, fmt.Sprintf(format, args...))
	}

	rows := groupRows(resp.Lines)
	if len(rows) == 0 {
		warn("OCR tidak menghasilkan teks")
		out.NeedsReview = true
		return out
	}

	col := detectPriceColumn(resp.Lines)
	b := &out.Bill
	b.Title = detectTitle(rows, medianHeight(resp.Lines))

	var (
		drafts          []itemDraft
		pending         []receiptRow // baris teks tanpa harga yang belum jelas milik siapa
		itemsClosed     bool         // true setelah bertemu subtotal/pajak/total/pembayaran
		orphan          rowKind      // label ringkasan tanpa angka; angkanya mungkin di baris berikutnya
		subtotal        int64
		itemDiscount    int64
		summaryDiscount int64
	)

	// Baris teks di antara item = lanjutan nama / catatan item sebelumnya ("Normal Ice", dll).
	flush := func() {
		if len(drafts) > 0 {
			for _, p := range pending {
				drafts[len(drafts)-1].name += " " + p.text()
			}
		}
		pending = nil
	}
	lowConf := func(c OCRLine) {
		if c.Score < 0.85 {
			warn("Angka %q dibaca dengan keyakinan rendah (%.2f)", c.Text, c.Score)
		}
	}

	for _, r := range rows {
		pi, price, hasPrice := r.priceIndex(col)
		kind := classify(r.label(pi))

		prevOrphan := orphan
		orphan = kindNone
		if hasPrice && len(r.cells) == 1 && prevOrphan != kindNone {
			kind = prevOrphan // baris berisi angka saja, labelnya ada di baris sebelumnya
		}
		if kind == kindDiscountWeak {
			if hasPrice && (price.neg || itemsClosed) {
				kind = kindDiscount
			} else {
				kind = kindNone
			}
		}
		summary := kind != kindNone

		switch {
		// --- Baris ringkasan berangka: subtotal, pajak, service, diskon, total, pembayaran ---
		case hasPrice && summary:
			flush()
			lowConf(r.cells[pi])
			switch kind {
			case kindDiscount:
				if itemsClosed {
					if summaryDiscount == 0 {
						summaryDiscount = price.value
					}
				} else {
					itemDiscount += price.value // diskon per item, masih di zona item
				}
			case kindSubtotal:
				itemsClosed = true
				if subtotal == 0 {
					subtotal = price.value
				}
			case kindTax:
				itemsClosed = true
				if b.TaxAmount == 0 {
					b.TaxAmount = price.value
				}
			case kindService:
				itemsClosed = true
				if b.ServiceAmount == 0 {
					b.ServiceAmount = price.value
				}
			case kindTotal:
				itemsClosed = true
				if b.ReceiptTotal == 0 { // "Total" pertama yang dipakai
					b.ReceiptTotal = price.value
				}
			case kindPayment:
				itemsClosed = true
			}

		// --- Baris berangka di zona item ---
		case hasPrice && !itemsClosed:
			lowConf(r.cells[pi])
			if price.neg { // angka negatif tanpa label dikenal = potongan
				flush()
				itemDiscount += price.value
				continue
			}
			others := r.cellsExcept(pi)
			d := itemDraft{qty: 1, printed: price.value}
			hasName := false
			for _, c := range others {
				if hasWordLetters(c.Text) {
					hasName = true
					break
				}
			}
			if hasName {
				// Satu baris: "1  Americano  17,000"
				flush()
				d.name, d.qty, d.unit = parseItemCells(others)
			} else {
				// Dua baris: nama di atas, rincian "2 x 12,000   24,000" di bawah
				d.qty, d.unit = parseDetail(others)
				if n := len(pending); n > 0 {
					d.name = pending[n-1].text()
					pending = pending[:n-1]
				}
				flush()
				if d.name == "" {
					d.name = "(tanpa nama)"
					warn("Ada harga %d tanpa nama item", d.printed)
				}
			}
			drafts = append(drafts, d)

		// --- Label ringkasan tanpa angka ---
		case !hasPrice && summary:
			flush()
			orphan = kind

		// --- Teks biasa (header, nama item, catatan) ---
		case !hasPrice && !itemsClosed:
			pending = append(pending, r)
		}
	}

	// =================================================================
	// Langkah 6: aritmetika sebagai juri
	// =================================================================
	b.DiscountAmount = summaryDiscount
	if b.DiscountAmount == 0 {
		b.DiscountAmount = itemDiscount
	}

	totalFits := func(sum int64) bool {
		if b.ReceiptTotal == 0 {
			return false
		}
		// Pajak/service bisa sudah termasuk di harga, atau ditambahkan di akhir.
		for _, extra := range []int64{0, b.TaxAmount, b.ServiceAmount, b.TaxAmount + b.ServiceAmount} {
			if sum+extra-b.DiscountAmount == b.ReceiptTotal {
				return true
			}
		}
		return false
	}
	fits := func(sum int64) bool { return (subtotal > 0 && sum == subtotal) || totalFits(sum) }

	var sumPrinted, sumUnit int64
	anyMulti := false
	for _, d := range drafts {
		sumPrinted += d.printed
		sumUnit += d.printed * int64(d.qty)
		if d.qty > 1 {
			anyMulti = true
		}
	}
	// Apakah angka di kolom harga itu total per baris (umum) atau harga satuan?
	unitMode := false
	if len(drafts) > 0 && !fits(sumPrinted) && anyMulti && fits(sumUnit) {
		unitMode = true
	}
	sum := sumPrinted
	if unitMode {
		sum = sumUnit
	}

	for _, d := range drafts {
		var unit int64
		switch {
		case d.unit > 0 && d.unit*int64(d.qty) == d.printed:
			unit = d.unit // rincian "2 x 12,000 = 24,000" cocok
		case unitMode:
			unit = d.printed
		default:
			unit = d.printed / int64(d.qty)
			if d.printed%int64(d.qty) != 0 {
				warn("Harga %d tidak habis dibagi qty %d pada item %q", d.printed, d.qty, d.name)
			}
		}
		out.Items = append(out.Items, model.BillItem{NameItem: d.name, Qty: d.qty, ItemPrice: unit})
	}

	if b.ReceiptTotal == 0 && subtotal > 0 {
		b.ReceiptTotal = subtotal + b.TaxAmount + b.ServiceAmount - b.DiscountAmount
		warn("Total tidak tertulis di struk; dihitung dari subtotal")
	}

	switch {
	case len(drafts) == 0:
		warn("Tidak ada item yang terdeteksi")
	case b.ReceiptTotal == 0:
		warn("Total tidak ditemukan")
	case !fits(sum):
		warn("Jumlah item (%d) tidak cocok dengan subtotal/total struk", sum)
	}
	if subtotal > 0 && b.ReceiptTotal > 0 && !totalFits(subtotal) {
		warn("Subtotal (%d) + pajak + service - diskon tidak sama dengan total (%d)", subtotal, b.ReceiptTotal)
	}

	out.NeedsReview = len(out.Warnings) > 0
	return out
}