//go:build windows

package i18n

import _ "embed"

//go:embed help_windows_vi.txt
var helpWinVI string

//go:embed help_windows_en.txt
var helpWinEN string

// On Windows the tokens live in Credential Manager and main's credential in
// Claude Code's own .credentials.json — every string naming the Keychain,
// /Applications, Cmd+V, `eval` or symlinks gets a Windows wording.
func init() {
	platform["vi"] = map[string]string{
		"__help__":       helpWinVI,
		"err_main_res":   "\"main\" là tên dành riêng cho acc đăng nhập bằng `claude auth login`.",
		"add_urlonly2":   " URL duyệt sẽ tự động được COPY VÀO CLIPBOARD (không tự mở browser). Dán (Ctrl+V) vào đúng cửa sổ/profile cần đăng nhập.",
		"add_default_b2": " Nếu acc \"%s\" nằm ở browser khác, dùng\n  --browser chrome | msedge | firefox | brave\nhoặc --url-only để tự dán URL.",
		"add_note":       "Sau khi bạn bấm Authorize, `claude setup-token` sẽ IN token ra màn hình.\nTrên Windows claude-acc chưa tự bắt được token đó: bôi đen và copy nó,\nrồi ở bước hỏi token chỉ cần nhấn Enter — token được lấy từ clipboard.",
		"add_nocatch":    "chưa tự bắt được token trên Windows.",
		"add_nocatch2":   " Copy token vừa in ra rồi nhấn Enter bên dưới (hoặc dán vào).",
		"err_keychain":   "không ghi được vào Windows Credential Manager.",
		"login_warn2":    " lệnh này GHI ĐÈ credential của Claude Code — acc \"main\" hiện tại",
		"rm_notfound":    "không tìm thấy token của \"%s\" trong Credential Manager (vẫn gỡ khỏi danh sách).",
		"ls_main_desc":   "credential của Claude Code — đầy đủ connectors, Remote Control",
		"ls_ws_summary":  "cả workspace: %s folder (%s dùng chung, %s có bản ghim riêng)",
		"run_glbpin":     "đang GHIM acc \"%s\" trong %%USERPROFILE%%\\.claude\\settings.json — nó đè lên `run`.\n     %s",
		"usage_env":      "dùng (PowerShell): claude-acc env <tên-acc|main> | Invoke-Expression",
		"un_tok":         "  - token Credential Manager của \"%s\"",
		"usg_nocred":     "không đọc được credential của main (.claude\\.credentials.json)",
		"sw_to_main2":    " (credential của Claude Code — đầy đủ connectors).",
		"sw_plain":       "Lưu ý: token nằm DẠNG VĂN BẢN trong %s.",
		"here_plain":     "Token nằm DẠNG VĂN BẢN trong %s.",
		"usage_here":     "dùng: claude-acc switch <tên-acc|main|off> --here [--at <thư-mục>]\n       main = repo này dùng acc chính · off = bỏ ghim riêng",
		"here_main2":     " (credential của Claude Code) cho: %s",
	}
	platform["en"] = map[string]string{
		"__help__":       helpWinEN,
		"err_main_res":   "\"main\" is reserved for the account signed in with `claude auth login`.",
		"add_urlonly2":   " the auth URL is COPIED TO YOUR CLIPBOARD instead of opening a browser. Paste it (Ctrl+V) into the window/profile you need.",
		"add_default_b2": " If account \"%s\" lives in another browser, use\n  --browser chrome | msedge | firefox | brave\nor --url-only to paste the URL yourself.",
		"add_note":       "After you click Authorize, `claude setup-token` PRINTS the token.\nOn Windows claude-acc cannot capture it yet: select and copy it, then just\npress Enter when asked for the token — it is taken from the clipboard.",
		"add_nocatch":    "the token is not captured automatically on Windows.",
		"add_nocatch2":   " Copy the token it printed, then press Enter below (or paste it).",
		"err_keychain":   "could not write to Windows Credential Manager.",
		"login_warn2":    " this OVERWRITES Claude Code's own credential — the current \"main\"",
		"rm_notfound":    "no Credential Manager token found for \"%s\" (removing it from the list anyway).",
		"ls_main_desc":   "Claude Code's own credential — full connectors, Remote Control",
		"ls_ws_summary":  "whole workspace: %s folders (%s shared, %s with their own copy of the pin)",
		"run_glbpin":     "account \"%s\" is pinned in %%USERPROFILE%%\\.claude\\settings.json — it overrides `run`.\n     %s",
		"usage_env":      "usage (PowerShell): claude-acc env <name|main> | Invoke-Expression",
		"un_tok":         "  - the Credential Manager token for \"%s\"",
		"usg_nocred":     "could not read main's credential (.claude\\.credentials.json)",
		"sw_to_main2":    " (Claude Code's own credential — full connectors).",
		"sw_plain":       "Note: the token sits in plain text in %s.",
		"here_plain":     "The token sits in plain text in %s.",
		"usage_here":     "usage: claude-acc switch <name|main|off> --here [--at <dir>]\n       main = this repo uses the main account · off = drop the pin",
		"here_main2":     " (Claude Code's own credential) to: %s",
	}
}
