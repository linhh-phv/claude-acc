# Thử claude-acc trên Windows

Hai phần: **(1) bộ tự kiểm** chạy tự động khoảng 3 phút và không động tới tài khoản thật;
**(2) vài bước tay** với tài khoản thật — những thứ không tự động được (OTP, đăng nhập thật,
extension trong Cursor/VS Code). Gặp gì khác với mô tả: chụp màn hình hoặc copy nguyên văn
chữ trong terminal, ghi số bước, rồi gửi lại. Không cần tự sửa gì.

## 0. Chuẩn bị

- Windows 10 (bản 1809 trở lên) hoặc Windows 11. Dùng **Windows Terminal** với PowerShell.
- Claude Code bản native đã cài và đã đăng nhập: `claude` chạy được. Tài khoản đang đăng nhập
  đó gọi là **main**. (Chưa cài cũng chạy được bộ tự kiểm, chỉ bỏ qua 2 bước dùng claude thật.)
- Nếu có `git` thì tốt — bộ tự kiểm cần nó cho phần ghim repo/workspace.
- Nhận 3 file, để chung một thư mục (máy ARM thì dùng bản `arm64`):
  - `claude-acc-windows-amd64.exe`
  - `claude-acc-selftest-windows-amd64.exe`
  - `install.ps1`

Nếu Windows/SmartScreen chặn file .exe: chuột phải → Properties → tick **Unblock**, hoặc chạy
`Unblock-File .\*.exe` trong PowerShell ở thư mục đó.

## 1. Bộ tự kiểm (bắt buộc, chạy trước)

```powershell
cd <thư mục chứa 3 file>
.\claude-acc-selftest-windows-amd64.exe -bin .\claude-acc-windows-amd64.exe
```

- Chạy khoảng 2–5 phút. Trong lúc chạy **đừng copy/dán gì** — nó dùng clipboard và trả lại
  nội dung cũ khi xong.
- Nó dùng một hồ sơ người dùng tạm, một `claude` giả và một API giả; token thử nằm trong
  Credential Manager dưới tên riêng và bị xoá khi xong. Tài khoản thật không bị đọc token,
  không bị ghi gì.
- Nếu máy có Claude Code, hai bước `TestRealClaude…` chạy `claude.exe` thật nhưng trong hồ sơ
  tạm và **dừng ở bước hỏi code**, không đăng nhập. Muốn bỏ qua hai bước đó: thêm `-no-real`.

**Đúng là:** dòng cuối là `PASS`. Dù kết quả thế nào, **gửi lại toàn bộ chữ in ra** (bôi đen
cả terminal rồi copy, hoặc chạy kèm `| Tee-Object selftest.txt` rồi gửi file
`selftest.txt`). Phần `TestThisMachine` ở đó cho biết phiên bản Windows, Claude Code, và
claude-acc có đọc được credential của main hay không (không in token ra).

## 2. Cài đặt

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install.ps1 -From .\claude-acc-windows-amd64.exe
```

Mở **terminal mới**, chạy `claude-acc version` → **Đúng là:** `claude-acc 2.0.0`.

## 3. Với tài khoản thật

**3a. Xem (chỉ đọc):**

```powershell
claude-acc ls
claude-acc check main
```

**Đúng là:** `ls` hiện `main` kèm email; `check main` kết thúc bằng `✓ main works`.

**3b. Thêm tài khoản thứ hai** (cần OTP gửi về email của tài khoản đó):

```powershell
claude-acc add thu
```

Trình duyệt mở → đăng nhập **tài khoản thứ hai** → nhập OTP → Authorize. Terminal **in ra token**
(chuỗi dài bắt đầu bằng `sk-ant-oat01-`): bôi đen, copy, rồi khi được hỏi token **chỉ nhấn
Enter**. **Đúng là:** `✓ "thu" is ready`.

Nếu thay vào đó trình duyệt hiện một **code** và terminal chờ ở `Paste code here if prompted >`:
dán code vào rồi Enter, và ghi lại chuyện này.

**3c. Đổi tài khoản:**

```powershell
claude-acc switch thu
claude -p "say hi"
claude-acc switch main
```

**Đúng là:** `switch thu` kết thúc bằng `✓ thu works`, `claude -p` trả lời bình thường.
Nếu có Cursor/VS Code với extension Claude Code: trong lúc đang `switch thu`, gửi một tin nhắn
trong khung chat — nó phải chạy bằng tài khoản `thu`, không cần Reload Window.

**3d. Đăng nhập lại main — tuỳ chọn, làm cuối cùng.** Lệnh này **ghi đè** đăng nhập main.

```powershell
claude-acc login
```

Ghi lại: trình duyệt có mở không; nếu trang hiện code, bấm **Copy** trên trang — claude-acc có
tự in `✓ picked up the code from the clipboard` rồi đăng nhập xong không; nếu dán tay thì chữ
có hiện ra không; cuối cùng có `✓ main works` không.

**3e. Dọn:**

```powershell
claude-acc rm thu
```

## Gửi lại

1. Toàn bộ output của bước 1.
2. Bước nào ở mục 3 khác với mô tả, kèm ảnh hoặc chữ.
