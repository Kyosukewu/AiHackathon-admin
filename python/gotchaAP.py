from selenium.webdriver.chrome.service import Service
from selenium.webdriver.common.by import By
from webdriver_manager.chrome import ChromeDriverManager
from dotenv import load_dotenv
import os
import json
import random
import time
import undetected_chromedriver as uc

def random_sleep(min_seconds=1.5, max_seconds=3.5):
    time.sleep(random.uniform(min_seconds, max_seconds))

def load_cookies(driver, cookies_path="ap_cookies.json"):
    with open(cookies_path, "r") as f:
        cookies = json.load(f)
    # 先打開主網域，才能正確設定 cookies
    driver.get("https://apvideohub.ap.org")
    random_sleep(1, 2)
    for cookie in cookies:
        cookie.pop("expiry", None)
        driver.add_cookie(cookie)

def init_driver():
    user_agent = (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/123.0.0.0 Safari/537.36"
    )
    options = uc.ChromeOptions()
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument(f"user-agent={user_agent}")
    driver = uc.Chrome(service=Service(ChromeDriverManager().install()), options=options)
    driver.execute_cdp_cmd("Page.addScriptToEvaluateOnNewDocument", {
        "source": """
            Object.defineProperty(navigator, 'webdriver', {
                get: () => undefined
            });
        """
    })
    return driver

def login():
    print("請選擇登入模式：")
    print("1. Cookies 登入")
    print("2. 自動登入（使用預設帳號密碼）")
    mode = input("請輸入模式 (1 或 2): ").strip()
    if mode == "2":
        # 自動登入流程，直接使用變數帶入帳號密碼，不再要求手動輸入
        print("🚀 開始自動登入流程...")
        driver = init_driver()
        login_url = "https://apvideohub.ap.org/login?returnUrl=%2F"
        driver.get(login_url)
        random_sleep(2, 3)
        username = os.getenv("USERNAME")
        password = os.getenv("PASSWORD")

        try:
            username_field = driver.find_element(By.CSS_SELECTOR, "#txt_username")
        except Exception as e:
            print("找不到使用者名稱的輸入欄位：", e)
            driver.quit()
            return None
        
        username_field.clear()
        username_field.send_keys(username)
        random_sleep(1, 2)

        try:
            password_field = driver.find_element(By.CSS_SELECTOR, "#txt_password")
        except Exception as e:
            print("找不到密碼的輸入欄位：", e)
            driver.quit()
            return None
        
        password_field.clear()
        password_field.send_keys(password)
        random_sleep(1, 2)

        try:
            login_button = driver.find_element(By.CSS_SELECTOR, "button.ap-btn.btn-primary.btn-lg.btn-block.m-0")
        except Exception as e:
            print("找不到登入按鈕：", e)
            driver.quit()
            return None
        login_button.click()
        random_sleep(2, 4)
        print("✅ 自動登入流程完成")
        return driver
    else:
        # 預設使用 cookies 登入模式
        print("🔑 開始登入流程，載入 cookies...")
        driver = init_driver()
        load_cookies(driver)
        random_sleep(2, 4)
        print("✅ 已完成登入流程")
        return driver

def goVideoPage(driver):
    # 導向影片搜尋頁面，利用 cookies 進入後台已認證的狀態
    driver.get("https://apvideohub.ap.org/home/hpsearch?id=5800bbf841784860b8ff60fb5dcb75d9")
    random_sleep(4, 6)

def download_videos(driver, limit):
    print("🔑 開始下載流程")
    # 一次性找尋並取出前 limit 筆下載按鈕
    download_buttons = driver.find_elements(By.CSS_SELECTOR, ".search-results video-tile ap-inline-download > i[title='Choose format']")
    if not download_buttons:
        print("⚠️ 找不到任何下載按鈕")
        return
    download_buttons = download_buttons[:limit]
    # 依序點擊所有下載按鈕，並模擬人類操作
    for download_btn in download_buttons:
        try:
            driver.execute_script("arguments[0].scrollIntoView({block: 'center'});", download_btn)
            driver.execute_script("arguments[0].click();", download_btn)
            random_sleep(1, 2)
        except Exception as e:
            print(f"❌ 點擊下載按鈕時發生錯誤: {e}")
            continue
    print("🎉 下載流程結束")

if __name__ == "__main__":
    driver = login()
    limit = 5
    try:
        goVideoPage(driver)
        download_videos(driver, limit)
    finally:
        pass