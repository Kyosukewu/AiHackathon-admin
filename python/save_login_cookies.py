import time
import json
import random
import undetected_chromedriver as uc
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager

def random_sleep(min_seconds=1.5, max_seconds=3.5):
    time.sleep(random.uniform(min_seconds, max_seconds))

def save_reuters_cookies():
    # 透過使用者輸入決定模式 (r: Reuters, a: AP)
    mode = input("請選擇模式 (r: Reuters, a: AP)：").strip().lower()
    if mode == "a":
        login_url = "https://apvideohub.ap.org/login?returnUrl=%2F"
        cookies_filename = "ap_cookies.json"
    else:
        login_url = "https://www.reutersconnect.com/login"
        cookies_filename = "reuters_cookies.json"

    # 🧑‍💻 偽裝真實使用者瀏覽器
    user_agent = (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/123.0.0.0 Safari/537.36"
    )

    options = uc.ChromeOptions()
    options.add_argument("--start-maximized")
    options.add_argument(f"user-agent={user_agent}")
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-blink-features=AutomationControlled")

    print("🚀 啟動瀏覽器...")
    driver = uc.Chrome(service=Service(ChromeDriverManager().install()), options=options)

    try:
        # ✅ 移除 navigator.webdriver 屬性
        driver.execute_cdp_cmd("Page.addScriptToEvaluateOnNewDocument", {
            "source": """
                Object.defineProperty(navigator, 'webdriver', {
                    get: () => undefined
                });
            """
        })

        print(f"🌐 使用以下網址開啟登入頁面：{login_url}")
        driver.get(login_url)

        # ⏳ 模擬人類等待
        random_sleep(3, 6)
        print("🔐 請手動登入（帳號密碼 + 驗證），登入成功後按 Enter 繼續...")

        # ✅ 等待使用者手動登入完畢
        input("🔔 登入完成後請按 Enter，我會儲存 cookies...")

        random_sleep(2, 4)

        # 🍪 取得並儲存 cookies
        cookies = driver.get_cookies()
        with open(cookies_filename, "w") as f:
            json.dump(cookies, f, indent=2)
        print(f"✅ 已儲存 cookies 到 {cookies_filename}")

    finally:
        random_sleep(1, 2)
        driver.quit()

if __name__ == "__main__":
    save_reuters_cookies()
