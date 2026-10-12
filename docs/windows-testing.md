# Thử claude-acc trên Windows — checklist

Bản Windows đã build được, nhưng **chưa ai chạy nó trên một máy Windows thật**. Checklist
này để người thử chạy theo thứ tự, ghi lại kết quả, rồi gửi lại cho người phát triển.

Mỗi bước có **"Đúng là"**: nếu thấy khác, chụp màn hình (hoặc copy nguyên văn chữ trong
terminal) và ghi số bước. Không cần sửa gì cả — chỉ cần báo lại.

Ba chỗ **chưa biết chắc**, được đánh dấu ⚠️ bên dưới — đó là những bước quan trọng nhất:

- Claude Code trên Windows có dùng biến `BROWSER` không (ảnh hưởng `--browser`,
  `--url-only`, và việc tự điền code khi `login`).
- Lúc `claude auth login` hỏi code, gõ chữ có hiện ra không.
- Ctrl+C khi đang chạy `claude` qua `claude-acc run`.

## 0. Chuẩn bị

- Windows 10 hoặc 11, dùng **Windows Terminal** (PowerShell). Nếu thử thêm được cmd.exe
  thì càng tốt.
- Claude Code đã cài bản native (`irm https://claude.ai/install.ps1 | iex`) và đã đăng
  nhập (`claude` chạy được). Tài khoản đang đăng nhập đó gọi là **main**.
- Một tài khoản Claude **thứ hai** (Pro/Max) để thử `add`. Lưu ý: bước `add` sẽ gửi mã
  OTP tới email của tài khoản đó.
- Nhận từ người phát triển: `claude-acc-windows-amd64.exe` (máy ARM thì
  `claude-acc-windows-arm64.exe`) và `install.ps1`.

Ghi lại: phiên bản Windows, `claude --version`, terminal đang dùng.

## 1. Cài đặt

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\install.ps1 -From .\claude-acc-windows-amd64.exe
```

Mở **terminal mới**, rồi:

```powershell
claude-acc version
```

**Đúng là:** in `claude-acc 2.0.0`.

## 2. Help, ngôn ngữ, hiển thị

```powershell
claude-acc help
claude-acc lang vi
claude-acc help
```

**Đúng là:**
- Bản help nói về **Windows Credential Manager** và **PowerShell**, không nhắc Keychain
  hay `eval`.
- Sau `lang vi`, chữ tiếng Việt có dấu hiển thị đúng, không bị vỡ thành ký tự lạ.
- Tiêu đề in đậm và có màu, không thấy các chuỗi kiểu `←[1m` lọt ra màn hình.

## 3. Xem trạng thái (chỉ đọc, an toàn)

```powershell
claude-acc ls
claude-acc check main
claude-acc usage main
```

**Đúng là:**
- `ls` hiện `main` kèm email của bạn.
- `check main` kết thúc bằng `✓ main dùng được`.
- `usage main` in một bảng có % cửa sổ 5h / 7 ngày và giờ reset.

## 4. Thêm tài khoản phụ

```powershell
claude-acc add thu
```

Trình duyệt mở ra → đăng nhập **tài khoản thứ hai** → nhập OTP → Authorize. Terminal sẽ
**in ra token** (chuỗi dài bắt đầu bằng `sk-ant-oat01-`). Bôi đen, copy token đó. Khi
claude-acc hỏi token, **chỉ cần nhấn Enter**.

**Đúng là:**
- Hiện `đã nhận: sk-ant-oat01-…` rồi `✓ "thu" sẵn sàng`.
- Mở *Control Panel → Credential Manager → Windows Credentials*: có mục
  `claude-code-token:thu`.

Nếu trình duyệt hiện một **code** và terminal chờ ở `Paste code here if prompted >`: dán
code đó vào rồi Enter. Ghi lại chuyện này có xảy ra hay không.

## 5. ⚠️ Chọn trình duyệt

```powershell
claude-acc add thu2 --browser msedge
```

Ghi lại: **Edge có mở ra không**, hay vẫn là trình duyệt mặc định? Huỷ bằng Ctrl+C nếu
không muốn thêm thật.

```powershell
claude-acc add thu2 --url-only
```

Ghi lại: trình duyệt có **không** mở ra không, và URL đăng nhập có nằm sẵn trong
clipboard không (thử Ctrl+V vào Notepad). Ctrl+C để huỷ.

## 6. Đổi tài khoản toàn cục

```powershell
claude-acc switch thu
claude -p "say hi"
claude-acc switch main
```

**Đúng là:**
- `switch thu` kết thúc bằng `✓ thu dùng được`.
- `claude -p` trả lời bình thường.
- Mở `%USERPROFILE%\.claude\settings.json`: khi đang ở `thu` thì có khối `env` chứa
  `CLAUDE_ACC_NAME: thu`, và sau `switch main` thì khối đó biến mất.

Nếu có Cursor hoặc VS Code với extension Claude Code: khi đang `switch thu`, gửi một tin
nhắn trong khung chat để xem nó có chạy bằng tài khoản `thu` không (không cần Reload
Window).

## 7. Ghim theo repo

Trong một repo git bất kỳ:

```powershell
cd C:\duong-dan\toi\mot-repo
claude-acc switch thu --here
claude-acc ls
git status
claude-acc switch off --here
```

**Đúng là:**
- Có file `.claude\settings.local.json` trong repo.
- `ls` có dòng `TẠI ĐÂY  thu`.
- `git status` **không** liệt kê file đó (nó đã được thêm vào `.git\info\exclude`).
- Sau `off`, khối `env` trong file đó biến mất.

Nếu có một workspace nhiều folder (file `.code-workspace`): đứng trong một folder **không
phải** folder đầu tiên và chạy `claude-acc switch thu --here --all`. **Đúng là:** mọi folder
đều có `.claude\settings.local.json` riêng, và `ls` gom chúng lại dưới folder đầu tiên.

## 8. ⚠️ Chạy một lần bằng tài khoản khác

```powershell
claude-acc run thu
```

Trong phiên claude vừa mở: gửi một câu hỏi dài, rồi nhấn **Ctrl+C** khi nó đang trả lời.
Sau đó gõ `/exit`.

**Đúng là:** Ctrl+C chỉ dừng câu trả lời, claude vẫn mở. Thoát xong thì quay lại dấu nhắc
PowerShell bình thường, gõ phím vẫn hiện chữ. Nếu terminal bị loạn (hai thứ cùng giành
bàn phím, chữ không hiện), hãy ghi lại.

## 9. Biến môi trường cho riêng phiên này

```powershell
claude-acc env thu | Invoke-Expression
$env:CLAUDE_CODE_OAUTH_TOKEN.Substring(0, 14)
claude-acc env main | Invoke-Expression
```

**Đúng là:** dòng giữa in `sk-ant-oat01-…`, và lệnh cuối xoá biến đó mà không báo lỗi.

## 10. ⚠️ Đăng nhập lại main — làm cuối cùng

Bước này **ghi đè** đăng nhập main hiện tại. Chỉ làm khi sẵn sàng đăng nhập lại main.

```powershell
claude-acc login
```

Ghi lại từng điểm:
- Trình duyệt có tự mở không?
- Nếu trình duyệt hiện **code**: bấm nút **Copy** trên trang. claude-acc có tự in
  `✓ đã lấy code từ clipboard` rồi đăng nhập xong không?
- Nếu không tự điền: dán code vào terminal. **Chữ có hiện ra khi dán không?** Enter có
  đăng nhập được không?
- Cuối cùng có `✓ main dùng được` không?

## 11. Dọn dẹp

```powershell
claude-acc rm thu
claude-acc uninstall
```

Ở câu hỏi `[y/N]` của `uninstall`, nhấn **Enter** (tức là không xoá). **Đúng là:** in
`đã hủy.`. Nếu muốn gỡ thật thì trả lời `y`.

## Gửi lại

- Danh sách số bước và kết quả: ✓ (đúng như mô tả), hoặc ✗ kèm ảnh/chữ.
- Đặc biệt là bước 5, 8 và 10.
- Thông tin ở mục 0: phiên bản Windows, `claude --version`, terminal đang dùng.
