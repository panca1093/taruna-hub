package service

import htmltemplate "html/template"

// emailPage is what an HTML email executes against: the params plus configuration-only values.
type emailPage struct {
	templateData
	Subject string
	AppURL  string
	Site    string
}

// emailLayout is the branded frame: tables and inline styles only, since mail clients drop <style>, fonts and scripts.
// Colours are hex copies of the web light-theme tokens. Each template defines preheader, eyebrow, content and footnote.
var emailLayout = htmltemplate.Must(htmltemplate.New("email").Parse(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light only">
<title>{{.Subject}}</title>
</head>
<body style="margin:0;padding:0;background:#f7f2ef;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;">{{template "preheader" .}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#f7f2ef;">
  <tr>
    <td align="center" style="padding:32px 16px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width:480px;">
        {{- with .AppURL}}
        <tr>
          <td align="center" style="padding:0 0 20px;">
            <img src="{{.}}/taruna-eduka-logo.png" width="180" height="60" alt="{{$.Get "tenant_name"}}" style="display:block;border:0;width:180px;height:60px;">
          </td>
        </tr>
        {{- end}}
        <tr>
          <td style="background:#fffdfa;border:1px solid #e7dfe0;border-radius:20px;padding:32px 28px;font-family:Inter,-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#3e3239;">
            <p style="margin:0 0 6px;font-size:13px;line-height:18px;letter-spacing:0.08em;text-transform:uppercase;color:#a87f94;font-weight:600;">{{template "eyebrow" .}}</p>
            <h1 style="margin:0 0 16px;font-size:22px;line-height:30px;font-weight:700;color:#3e3239;">Halo {{.Get "name"}},</h1>
            {{template "content" .}}
            <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td style="padding:24px 0 0;"><div style="border-top:1px solid #e7dfe0;font-size:0;line-height:0;">&nbsp;</div></td></tr></table>
            <p style="margin:16px 0 0;font-size:13px;line-height:20px;color:#786771;">{{template "footnote" .}}</p>
          </td>
        </tr>
        <tr>
          <td align="center" style="padding:20px 12px 0;font-family:Inter,-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:12px;line-height:18px;color:#786771;">
            Email ini dikirim otomatis oleh {{.Get "tenant_name"}}, mohon tidak dibalas.
            {{- with .AppURL}}<br>
            <a href="{{.}}" style="color:#6b455d;text-decoration:underline;">{{$.Site}}</a>
            {{- end}}
          </td>
        </tr>
      </table>
    </td>
  </tr>
</table>
</body>
</html>
{{define "lead"}}<p style="margin:0 0 20px;font-size:16px;line-height:24px;color:#3e3239;">{{.}}</p>{{end}}
{{define "details-open"}}<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td style="background:#eee1e6;border-radius:14px;padding:16px 20px;font-size:15px;line-height:24px;color:#3e3239;">{{end}}
{{define "details-close"}}</td></tr></table>{{end}}
`))

// emailHTML is the HTML alternative per template; a template missing here is sent as plain text only.
var emailHTML = map[string]string{
	TemplateEmailCode: `{{define "preheader"}}Kode verifikasi Anda: {{.Get "code"}}. Berlaku {{.Get "minutes"}} menit.{{end}}
{{define "eyebrow"}}Verifikasi email{{end}}
{{define "content"}}<p style="margin:0 0 20px;font-size:16px;line-height:24px;color:#3e3239;">Masukkan kode ini untuk menyelesaikan pendaftaran akun {{.Get "tenant_name"}} Anda.</p>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" style="background:#eee1e6;border-radius:14px;padding:20px 12px;">
<span style="font-family:'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace;font-size:34px;line-height:40px;font-weight:700;letter-spacing:0.3em;color:#6b455d;padding-left:0.3em;">{{.Get "code"}}</span>
</td></tr></table>
<p style="margin:20px 0 0;font-size:15px;line-height:22px;color:#3e3239;">Kode berlaku <strong>{{.Get "minutes"}} menit</strong>. Jangan bagikan kode ini kepada siapa pun, termasuk pihak yang mengaku dari {{.Get "tenant_name"}}.</p>{{end}}
{{define "footnote"}}Abaikan email ini jika Anda tidak merasa mendaftar.{{end}}`,

	TemplatePasswordReset: `{{define "preheader"}}Tautan atur ulang kata sandi berlaku {{.Get "minutes"}} menit.{{end}}
{{define "eyebrow"}}Atur ulang kata sandi{{end}}
{{define "content"}}<p style="margin:0 0 20px;font-size:16px;line-height:24px;color:#3e3239;">Tekan tombol berikut untuk mengatur ulang kata sandi akun {{.Get "tenant_name"}} Anda.</p>
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td style="background:#6b455d;border-radius:12px;">
<a href="{{.Get "reset_url"}}" style="display:inline-block;padding:14px 28px;font-size:16px;line-height:20px;font-weight:600;color:#fffdfa;text-decoration:none;border-radius:12px;">Atur ulang kata sandi</a>
</td></tr></table>
<p style="margin:20px 0 0;font-size:15px;line-height:22px;color:#3e3239;">Tautan berlaku <strong>{{.Get "minutes"}} menit</strong> dan hanya dapat dipakai sekali.</p>
<p style="margin:16px 0 0;font-size:13px;line-height:20px;color:#786771;">Jika tombol tidak berfungsi, salin tautan ini ke browser:<br>
<a href="{{.Get "reset_url"}}" style="color:#6b455d;word-break:break-all;">{{.Get "reset_url"}}</a></p>{{end}}
{{define "footnote"}}Abaikan email ini jika Anda tidak memintanya; kata sandi Anda tidak berubah.{{end}}`,

	TemplateOrderConfirmed: `{{define "preheader"}}Pesanan {{.Get "code"}} sudah terkonfirmasi.{{end}}
{{define "eyebrow"}}Pesanan terkonfirmasi{{end}}
{{define "content"}}{{template "lead" "Pesanan kamu sudah terkonfirmasi. Akses tryout kamu kini terbuka di menu Paket Belajar Saya."}}
{{template "details-open"}}Kode pesanan: <strong>{{.Get "code"}}</strong><br>Produk: <strong>{{.Get "product"}}</strong>{{template "details-close"}}{{end}}
{{define "footnote"}}Salam belajar,<br>{{.Get "tenant_name"}}{{end}}`,

	TemplateOrderCancelled: `{{define "preheader"}}Pesanan {{.Get "code"}} dibatalkan.{{end}}
{{define "eyebrow"}}Pesanan dibatalkan{{end}}
{{define "content"}}{{template "lead" "Pesanan kamu dibatalkan dan akses tryout tidak dibuka."}}
{{template "details-open"}}Kode pesanan: <strong>{{.Get "code"}}</strong><br>Produk: <strong>{{.Get "product"}}</strong>
{{- with .Get "reason"}}<br>Alasan: <strong>{{.}}</strong>{{end}}{{template "details-close"}}{{end}}
{{define "footnote"}}Hubungi admin {{.Get "tenant_name"}} jika ada pertanyaan.{{end}}`,

	TemplateTopupApproved: `{{define "preheader"}}Top-up {{.Get "code"}} sebesar {{.Get "amount"}} sudah disetujui.{{end}}
{{define "eyebrow"}}Top-up disetujui{{end}}
{{define "content"}}{{template "lead" "Top-up kamu sudah disetujui. Saldo dompet kamu sudah bertambah."}}
{{template "details-open"}}Kode top-up: <strong>{{.Get "code"}}</strong><br>Jumlah: <strong>{{.Get "amount"}}</strong>{{template "details-close"}}{{end}}
{{define "footnote"}}Salam belajar,<br>{{.Get "tenant_name"}}{{end}}`,

	TemplateTopupRejected: `{{define "preheader"}}Top-up {{.Get "code"}} ditolak.{{end}}
{{define "eyebrow"}}Top-up ditolak{{end}}
{{define "content"}}{{template "lead" "Top-up kamu ditolak dan saldo dompet tidak berubah."}}
{{template "details-open"}}Kode top-up: <strong>{{.Get "code"}}</strong><br>Jumlah: <strong>{{.Get "amount"}}</strong>
{{- with .Get "reason"}}<br>Alasan: <strong>{{.}}</strong>{{end}}{{template "details-close"}}{{end}}
{{define "footnote"}}Hubungi admin {{.Get "tenant_name"}} jika ada pertanyaan.{{end}}`,
}
