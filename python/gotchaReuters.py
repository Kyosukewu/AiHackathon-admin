import json
import time
import random
import undetected_chromedriver as uc
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager
from selenium.webdriver.common.action_chains import ActionChains

def random_sleep(min_seconds=1.5, max_seconds=3.5):
    time.sleep(random.uniform(min_seconds, max_seconds))

def load_cookies(driver, cookies_path="reuters_cookies.json"):
    with open(cookies_path, "r") as f:
        cookies = json.load(f)

    driver.get("https://www.reutersconnect.com")  # 設 cookies 前要先載入主網域
    random_sleep(1, 2)

    for cookie in cookies:
        cookie.pop("expiry", None)
        driver.add_cookie(cookie)

def visit_first_video():
    print("🚀 啟動瀏覽器...")

    user_agent = (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/123.0.0.0 Safari/537.36"
    )

    options = uc.ChromeOptions()
    options.add_argument("--start-maximized")
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument(f"user-agent={user_agent}")

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

        print("🌐 導向首頁並載入 cookies...")
        load_cookies(driver)
        random_sleep(2, 4)

        print("🔄 導向影片列表頁面...")
        driver.get("https://www.reutersconnect.com/all?media-types=vid")
        random_sleep(4, 6)

        print("🔍 尋找第一筆影片...")
        ol = driver.find_element(By.CSS_SELECTOR, "ol.items-grid")
        first_li = ol.find_element(By.CSS_SELECTOR, "li")

        # 滑鼠移動模擬（更像人）
        a_tag = first_li.find_element(By.TAG_NAME, "a")
        ActionChains(driver).move_to_element(a_tag).pause(1.2).click().perform()

        print("✅ 已點擊第一筆影片，等待頁面載入中...")
        random_sleep(10, 15)

    finally:
        driver.quit()

if __name__ == "__main__":
    visit_first_video()
