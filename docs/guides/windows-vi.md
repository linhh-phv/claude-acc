# Dùng claude-acc trên Windows

claude-acc giúp dùng **nhiều tài khoản Claude Code trên một máy** và đổi qua lại bằng một
lệnh — cho cả terminal lẫn Cursor/VS Code — không phải đăng nhập lại.

## Cần có

- Windows 10 hoặc 11.
- Claude Code đã cài và **đã đăng nhập** (gõ `claude` trong PowerShell chạy được).
  Tài khoản đang đăng nhập đó gọi là **main**.

## 1. Cài đặt (làm 1 lần)

Mở PowerShell (bấm Start → gõ `PowerShell` → Enter), dán lệnh này rồi Enter:

```powershell
irm https://raw.githubusercontent.com/linhh-phv/claude-acc/main/install.ps1 | iex
```

Mở **một cửa sổ PowerShell mới**, gõ:

```powershell
claude-acc version
```

Thấy `claude-acc 2.0.2` là cài xong.

Không muốn dán lệnh? Tải `claude-acc-windows-amd64.zip` (máy chip ARM: `…-arm64.zip`) ở
[bản phát hành mới nhất](https://github.com/linhh-phv/claude-acc/releases/latest), giải nén,
rồi bấm đúp **`install.cmd`**. Nếu Windows hiện *"Windows protected your PC"*: bấm
**More info → Run anyway**.

Muốn thông báo bằng tiếng Việt: `claude-acc lang vi`.

## 2. Thêm một tài khoản (làm 1 lần cho mỗi tài khoản)

Ví dụ thêm tài khoản đặt tên `cong-viec`:

```powershell
claude-acc add cong-viec
```

1. Trình duyệt mở ra. Đăng nhập **đúng tài khoản muốn thêm**.
   Nếu trình duyệt đang đăng nhập tài khoản khác, chạy
   `claude-acc add cong-viec --url-only` thay vào đó: link đăng nhập được copy sẵn,
   dán nó vào một **cửa sổ ẩn danh** (Ctrl+Shift+N).
2. Nhập mã OTP gửi về email của tài khoản đó, rồi bấm **Authorize**.
3. Terminal in ra token — một chuỗi dài bắt đầu bằng `sk-ant-oat01-`.
   **Bôi đen cả chuỗi đó rồi nhấn Ctrl+C.**
4. Khi claude-acc hỏi token: **chỉ cần nhấn Enter** (nó tự lấy từ clipboard).
5. Thấy `✓ "cong-viec" sẵn sàng.` (hoặc `is ready.`) là xong.

## 3. Dùng hằng ngày

| Muốn | Gõ |
|---|---|
| Đổi sang `cong-viec` cho cả terminal lẫn Cursor/VS Code | `claude-acc switch cong-viec` |
| Quay về tài khoản chính | `claude-acc switch main` |
| Chỉ repo đang mở dùng `cong-viec` | `cd <thư-mục-repo>` rồi `claude-acc switch cong-viec --here` |
| Bỏ ghim riêng của repo đó | `claude-acc switch off --here` |
| Xem tài khoản nào đang có hiệu lực, ở đâu | `claude-acc ls` |
| Xem mỗi tài khoản còn bao nhiêu hạn mức | `claude-acc usage` |
| Xoá một tài khoản khỏi claude-acc | `claude-acc rm cong-viec` |
| Xem toàn bộ lệnh | `claude-acc help` |

Đổi tài khoản là **có hiệu lực ngay** — không cần mở lại Cursor/VS Code, không cần mở tab
chat mới.

Muốn một repo dùng tài khoản **main** thì để main là tài khoản chung (`claude-acc switch
main`), rồi ghim tài khoản phụ cho các repo khác. Claude Code từ bản 2.1.312 không cho một
repo dùng main khi tài khoản chung đang là tài khoản phụ — claude-acc sẽ báo khi gặp trường
hợp đó. Nếu `claude-acc ls` nhắc chạy `claude-acc sync` dưới một repo đã ghim (ghim làm từ
bản claude-acc cũ), chạy lệnh đó một lần.

Lưu ý: chỉ tài khoản **main** có claude.ai connectors và Remote Control; các tài khoản thêm
bằng `add` dùng token, chạy Claude Code bình thường nhưng không có hai thứ đó.

## Gặp lỗi

Chụp màn hình (hoặc bôi đen copy nguyên chữ trong cửa sổ PowerShell), kèm kết quả của

```powershell
claude-acc version
claude --version
```

rồi gửi cho người đã đưa bạn claude-acc.
