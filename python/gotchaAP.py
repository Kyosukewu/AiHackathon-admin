import os
import json
import random
import time
import re
from urllib.parse import urljoin
import shutil

from selenium.webdriver.chrome.service import Service
from selenium.webdriver.common.by import By
from webdriver_manager.chrome import ChromeDriverManager
from dotenv import load_dotenv
import undetected_chromedriver as uc
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.common.exceptions import TimeoutException, NoSuchElementException

# --- 全域常數 ---
BASE_URL = "https://apvideohub.ap.org"
LOGIN_URL = f"{BASE_URL}/login?returnUrl=%2F"
SEARCH_PAGE_URL_TEMPLATE = f"{BASE_URL}/home/hpsearch?id={{hpSectionId}}" # 範例，如果需要動態ID
DEFAULT_SEARCH_ID = "5800bbf841784860b8ff60fb5dcb75d9" # 您目前使用的ID

USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/123.0.0.0 Safari/537.36"
)

# 檔案路徑相關常數
DOWNLOAD_BASE_DIR = "Download"
AP_SUB_DIR = "ap"
TEMP_VIDEOS_SUB_DIR = "temp_videos"
TXT_SUB_DIR = "" # TXT 會直接在 {video_id} 下，符合 Download/ap/{video_id}/{video_id}.txt
DEBUG_BASE_DIR = "Debug"
GENERAL_ERRORS_DIR_NAME = "general_errors"


# --- 通用輔助函式 ---

def random_sleep(min_seconds=1.0, max_seconds=2.5):
    """產生隨機秒數的延遲。"""
    time.sleep(random.uniform(min_seconds, max_seconds))

def save_debug_info(driver, step_name, video_id=None):
    """儲存截圖和頁面源碼到分類資料夾。"""
    if video_id and video_id != "unknown" and not str(video_id).startswith("unknown_") and not str(video_id).startswith("loop_"):
        debug_dir = os.path.join(DEBUG_BASE_DIR, str(video_id))
    else:
        debug_dir = os.path.join(DEBUG_BASE_DIR, GENERAL_ERRORS_DIR_NAME)
    
    print(f"   -> 正在儲存除錯資訊至: {debug_dir} (步驟: {step_name})")
    os.makedirs(debug_dir, exist_ok=True)
    
    full_screenshot_path = os.path.join(debug_dir, f"{step_name}.png")
    full_source_path = os.path.join(debug_dir, f"{step_name}.html")
    
    try:
        driver.save_screenshot(full_screenshot_path)
        print(f"   -> 截圖已儲存: {full_screenshot_path}")
    except Exception as e:
        print(f"   -> 儲存截圖失敗: {e}")
    try:
        with open(full_source_path, "w", encoding="utf-8") as f:
            f.write(driver.page_source)
        print(f"   -> 頁面源碼已儲存: {full_source_path}")
    except Exception as e:
        print(f"   -> 儲存頁面源碼失敗: {e}")

# --- 初始化與登入 ---

def init_driver():
    """初始化 WebDriver 並設定反偵測選項與下載路徑。"""
    print("🚀 正在初始化 WebDriver...")
    options = uc.ChromeOptions()
    options.add_argument("--no-sandbox")
    options.add_argument(f"user-agent={USER_AGENT}")
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument("--start-maximized")

    # 設定預設下載路徑
    # 所有影片都會先下載到 TEMP_VIDEOS_SUB_DIR
    default_download_path = os.path.join(os.getcwd(), DOWNLOAD_BASE_DIR, AP_SUB_DIR, TEMP_VIDEOS_SUB_DIR)
    os.makedirs(default_download_path, exist_ok=True)
    print(f"📦 設定預設下載路徑為: {os.path.abspath(default_download_path)}")
    
    prefs = {
        "download.default_directory": os.path.abspath(default_download_path),
        "download.prompt_for_download": False,
        "download.directory_upgrade": True,
        "safeBrowse.enabled": True,
        "profile.default_content_setting_values.automatic_downloads": 1 # 允許自動下載多個檔案
    }
    options.add_experimental_option("prefs", prefs)
    
    try:
        driver = uc.Chrome(service=Service(ChromeDriverManager().install()), options=options)
        driver.execute_cdp_cmd(
            "Page.addScriptToEvaluateOnNewDocument",
            {"source": """Object.defineProperty(navigator, 'webdriver', { get: () => undefined });"""}
        )
        print("✅ WebDriver 初始化完成。")
        return driver
    except Exception as e:
        print(f"❌ 初始化 WebDriver 失敗: {e}")
        return None

def login(driver):
    """
    處理登入邏輯 (僅使用 .env 的 AP_USERNAME 和 AP_PASSWORD)。
    Args:
        driver: Selenium WebDriver 實例。
    Returns:
        True 如果登入成功, False 如果失敗。
    """
    load_dotenv() # 確保 .env 檔案被載入
    print("🚀 開始自動登入流程 (使用 AP_USERNAME 和 AP_PASSWORD)...")
    driver.get(LOGIN_URL)
    random_sleep(2, 3)

    username = os.getenv("AP_USERNAME")
    password = os.getenv("AP_PASSWORD")

    if not username or not password:
        print("❌ 請確保 .env 檔案中已設定 AP_USERNAME 和 AP_PASSWORD。")
        return False

    try:
        WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "#txt_username"))
        ).send_keys(username)
        
        password_field = driver.find_element(By.CSS_SELECTOR, "#txt_password")
        password_field.clear()
        password_field.send_keys(password)
        random_sleep(0.5, 1)

        login_button = driver.find_element(By.CSS_SELECTOR, "button.ap-btn.btn-primary.btn-lg.btn-block.m-0")
        login_button.click()
        random_sleep(3, 5)

        # 驗證登入是否成功 (例如，檢查某個登入後才會出現的元素)
        WebDriverWait(driver, 15).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "app-header")) 
        )
        print("✅ 自動登入流程完成。")
        return True
    except Exception as e:
        print(f"❌ 自動登入失敗: {e}")
        save_debug_info(driver, "login_error")
        return False

# --- 頁面互動與資料處理輔助函式 ---

def handle_onetrust_popup(driver, banner_timeout=4, button_timeout=3):
    """檢查並嘗試接受 OneTrust Cookie 彈窗，使用較短等待時間。"""
    selectors_to_try = [
        "#onetrust-accept-btn-handler",
        "#accept-recommended-btn-handler",
        "button.save-preference-btn-handler"
    ]
    print(f"⏳ 檢查 OneTrust Cookie Banner (容器最多等待 {banner_timeout}s, 按鈕最多等待 {button_timeout}s)...")
    clicked = False
    banner_container_selector = "#onetrust-consent-sdk" 
    
    try:
        WebDriverWait(driver, banner_timeout).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, banner_container_selector))
        )
        print("   -> 偵測到 OneTrust Banner 容器。")

        for selector in selectors_to_try:
            try:
                button = WebDriverWait(driver, button_timeout).until(
                    EC.element_to_be_clickable((By.CSS_SELECTOR, selector))
                )
                print(f"   -> 發現按鈕 ({selector})，嘗試點擊...")
                driver.execute_script("arguments[0].click();", button)
                print("   -> 已點擊。")
                random_sleep(1.0, 1.5)
                clicked = True
                break 
            except TimeoutException:
                pass 
            except Exception as e_click:
                print(f"   -> 點擊按鈕 {selector} 時發生其他錯誤: {e_click}")
                pass 
                
    except TimeoutException:
        print(f"   -> 在 {banner_timeout}s 內未偵測到 OneTrust Banner 容器。")
    except Exception as e_banner:
        print(f"   -> 檢查 OneTrust Banner 容器時發生錯誤: {e_banner}")

    if clicked: 
        print("✅ OneTrust Banner 已處理。")
    else: 
        print("   -> 未處理 OneTrust Banner (可能不存在或未在時限內找到可點擊按鈕)。")

def try_click_next_story(driver):
    """嘗試點擊 'Next Story' 按鈕。"""
    try:
        print("⏳ 嘗試點擊 'Next Story'...")
        next_button = WebDriverWait(driver, 10).until(
            EC.element_to_be_clickable((By.ID, "nextStory"))
        )
        driver.execute_script("arguments[0].click();", next_button)
        print("   -> 已點擊 'Next Story'。")
        random_sleep(4, 6) # 等待頁面跳轉和載入
        return True
    except Exception as e:
        print(f"❌ 點擊 Next Story 時發生錯誤: {e}。")
        return False

def verify_page_elements_visible(driver, wait_timeout=45):
    """確認影片詳情頁的核心元素（#videoContent 和 Video ID 位置）已載入。"""
    print("⏳ 等待影片詳情頁核心元素可見...")
    try:
        WebDriverWait(driver, wait_timeout).until(
            EC.visibility_of_element_located((By.ID, "videoContent"))
        )
        print("✅ #videoContent 已可見。")
        
        id_value_xpath = "//td[contains(text(), 'Video ID:')]/following-sibling::td[1]"
        WebDriverWait(driver, 20).until(
            EC.presence_of_element_located((By.XPATH, id_value_xpath))
        )
        print(f"✅ Video ID 位置 ({id_value_xpath}) 已存在。")
        return True
    except TimeoutException:
        print("❌ 等待核心元素超時。")
        return False

def extract_video_metadata(driver):
    """擷取 Video ID 和 Title。返回包含 'id' 和 'title' 的字典。"""
    video_id = f"unknown_{int(time.time())}" # 預設ID
    title_text = "N/A" # 預設標題
    
    id_value_xpath = "//td[contains(text(), 'Video ID:')]/following-sibling::td[1]"
    try:
        video_id_element = driver.find_element(By.XPATH, id_value_xpath)
        extracted_id = video_id_element.text.strip()
        if extracted_id and extracted_id.isdigit():
            video_id = extracted_id
            print(f"🆔 取得 Video ID: {video_id}")
        else:
            print(f"   ⚠️ 取得的 Video ID '{extracted_id}' 格式不正確，使用預設值 {video_id}。")
    except Exception as e:
        print(f"❌ 擷取 Video ID 失敗: {e}")

    try:
        title_element = driver.find_element(By.CSS_SELECTOR, "#videoContent h2.ap-sans-bold")
        title_text = title_element.text.strip()
        print(f"👑 取得 Title: {title_text}")
    except Exception as e:
        print(f"⚠️ 擷取 Title 失敗: {e}")
    
    return {"id": video_id, "title": title_text}

def click_main_download_button(driver):
    """點擊主下載按鈕。返回 True 表示成功嘗試點擊，False 表示失敗。"""
    try:
        download_selector = "ap-inline-download button.download-clp"
        print(f"⏳ 尋找並點擊下載按鈕 ({download_selector})...")
        download_button = WebDriverWait(driver, 15).until(
            EC.element_to_be_clickable((By.CSS_SELECTOR, download_selector))
        )
        print("   -> 按鈕已變為可點擊。")
        driver.execute_script("arguments[0].scrollIntoView({block: 'center', inline: 'nearest'});", download_button)
        time.sleep(0.5) 
        driver.execute_script("arguments[0].click();", download_button)
        print("🖱️ 已嘗試點擊下載按鈕。")
        return True
    except Exception as e:
        print(f"⚠️ 點擊下載按鈕失敗: {e}。")
        return False

def wait_for_download_and_move(video_id, default_download_dir, target_base_dir, 
                               timeout_seconds=7200, check_interval=5, stable_checks_required=4):
    """等待下載完成並移動檔案。"""
    print(f"⏳ (Video ID: {video_id}) 等待影片下載完成 (最長 {timeout_seconds // 60} 分鐘)...")
    print(f"   -> 監控下載資料夾: {default_download_dir}")

    start_time = time.time()
    potential_files_info = {} # {filename: {'size': size, 'stable_checks': count, 'path': path, 'last_seen': timestamp}}
    processed_files = set() 

    while time.time() - start_time < timeout_seconds:
        try:
            if not os.path.exists(default_download_dir):
                print(f"   -> 監控的下載資料夾 {default_download_dir} 不存在。")
                return False

            current_files_in_dir = os.listdir(default_download_dir)
            
            for filename in current_files_in_dir:
                if filename.lower().endswith(('.crdownload', '.tmp', '.part')) or \
                   filename.startswith('.') or filename in processed_files:
                    potential_final_name = filename.split('.crdownload')[0].split('.part')[0].split('.tmp')[0]
                    if potential_final_name in potential_files_info:
                         potential_files_info[potential_final_name]['stable_checks'] = 0 # 重置穩定性
                    continue

                file_path = os.path.join(default_download_dir, filename)
                try:
                    current_size = os.path.getsize(file_path)
                    current_time_file = time.time()

                    if filename not in potential_files_info:
                        if current_size > 0: 
                            print(f"   -> 發現潛在檔案: {filename} (大小: {current_size} bytes)")
                            potential_files_info[filename] = {'size': current_size, 'stable_checks': 1, 'path': file_path, 'last_seen': current_time_file}
                        else: # 可能是下載剛開始，檔案大小為0
                            potential_files_info[filename] = {'size': 0, 'stable_checks': 0, 'path': file_path, 'last_seen': current_time_file}
                    else:
                        # 更新檔案最後被檢查的時間
                        potential_files_info[filename]['last_seen'] = current_time_file
                        if potential_files_info[filename]['size'] == current_size and current_size > 0:
                            potential_files_info[filename]['stable_checks'] += 1
                            if potential_files_info[filename]['stable_checks'] >= stable_checks_required:
                                print(f"✅ 偵測到下載完成的檔案: {filename} (大小: {current_size} bytes)")
                                source_path = potential_files_info[filename]['path']
                                
                                target_video_dir = os.path.join(target_base_dir, str(video_id))
                                os.makedirs(target_video_dir, exist_ok=True)
                                
                                _, file_extension = os.path.splitext(filename)
                                new_filename = f"{video_id}{file_extension}"
                                target_file_path = os.path.join(target_video_dir, new_filename)
                                
                                try:
                                    print(f"   -> 正在移動檔案從 {source_path} 至 {target_file_path}")
                                    shutil.move(source_path, target_file_path)
                                    print(f"📦 影片已成功移動至: {target_file_path}")
                                    processed_files.add(filename) 
                                    del potential_files_info[filename] 
                                    return True 
                                except Exception as e:
                                    print(f"❌ 移動檔案 '{filename}' 失敗: {e}")
                                    print(f"   -> 檔案仍位於: {source_path}")
                                    processed_files.add(filename) 
                                    del potential_files_info[filename] 
                                    return False 
                        else: # 大小改變或仍為0
                            if current_size > 0 and filename in potential_files_info and potential_files_info[filename]['size'] != current_size :
                                print(f"   -> 檔案 {filename} 大小已改變: {potential_files_info[filename]['size']} -> {current_size}")
                            potential_files_info[filename]['size'] = current_size
                            potential_files_info[filename]['stable_checks'] = 0
                except FileNotFoundError: 
                    if filename in potential_files_info: del potential_files_info[filename]
                    continue
            
            # 清理長時間未更新的追蹤 (避免舊檔案影響)
            current_time_for_cleanup = time.time()
            # 使用 list(potential_files_info.items()) 避免在迭代時修改字典
            stale_files = [
                fn for fn, info in list(potential_files_info.items()) 
                if current_time_for_cleanup - info.get('last_seen', 0) > (check_interval * (stable_checks_required + 5)) # 增加清理閾值
            ]
            for fn in stale_files:
                print(f"   -> 檔案 {fn} 長時間未變化且非穩定，停止追蹤。")
                del potential_files_info[fn]

        except Exception as e:
            print(f"   -> 監控下載資料夾時發生錯誤: {e}")

        print(f"   -> 已等待 {int(time.time() - start_time)} 秒... (監控中)")
        time.sleep(check_interval)

    print(f"⚠️ 等待下載超時 ({timeout_seconds} 秒) 或未找到穩定檔案。")
    return False

def extract_and_save_text_details(driver, video_id, title_text, txt_base_output_dir):
    """擷取、處理並儲存影片的文字描述。"""
    print("📝 準備擷取和儲存文字詳情...")
    content_text = ""
    try:
        video_content_element = driver.find_element(By.ID, "videoContent")
        content_text = video_content_element.get_attribute('innerText').strip()
        print(f"   -> 擷取到 #videoContent 內容 (原始長度: {len(content_text)})")
    except Exception as e:
        print(f"❌ 擷取 #videoContent 失敗: {e}")

    content_text_no_title = content_text
    if title_text != "N/A" and content_text and content_text.startswith(title_text):
        # 移除標題行以及可能跟隨的一個換行符
        content_text_no_title = content_text[len(title_text):].lstrip('\n').strip()
        print("   -> 已從內文中移除標題行。")

    separator = "==========================================================="
    separator_index = content_text_no_title.find(separator)
    processed_content = content_text_no_title[:separator_index].strip() if separator_index != -1 else content_text_no_title.strip()
    if separator_index != -1:
        print(f"   -> 已移除 '{separator}' 之後的內容。")

    final_text_to_save = f"Title: {title_text}\n\n{processed_content}"

    if processed_content or title_text != "N/A": # 即使內文處理後為空，但有標題也儲存
        # 建立 Download/ap/{video_id}/
        txt_video_id_dir = os.path.join(txt_base_output_dir, str(video_id))
        os.makedirs(txt_video_id_dir, exist_ok=True)
        
        file_path = os.path.join(txt_video_id_dir, f"{video_id}.txt")
        with open(file_path, "w", encoding="utf-8") as f:
            f.write(final_text_to_save)
        print(f"💾 文字檔案已儲存至: {file_path}")
    else:
        print(f"⚠️ 處理後內容與標題皆為空，未儲存文字檔案。")

def navigate_to_next_video_page(driver, current_loop_index, total_limit):
    """導航到下一個影片（如果需要），並處理彈窗。"""
    if current_loop_index < total_limit - 1:
        if try_click_next_story(driver): # 使用更名後的函式
            handle_onetrust_popup(driver) 
            random_sleep(1, 2)
            return True 
        else:
            print("❌ 無法點擊 Next Story，流程中止。")
            return False 
    else:
        print("🏁 已達到處理上限。")
        return False 

def check_if_id_processed(video_id, base_data_path="Download/ap"):
    """檢查影片 ID 對應的資料夾是否存在，若存在則表示已處理過。"""
    if not video_id or str(video_id).startswith("unknown_") or str(video_id).startswith("loop_"):
        return False 
    
    video_specific_dir = os.path.join(base_data_path, str(video_id))
    if os.path.isdir(video_specific_dir):
        print(f"   -> 目錄 {video_specific_dir} 已存在。")
        return True
    return False

# --- 主流程函式 ---
def go_to_search_page(driver):
    """導向影片搜尋頁面。"""
    print("🚗 正在導向影片搜尋頁面...")
    search_url = SEARCH_PAGE_URL_TEMPLATE.format(hpSectionId=DEFAULT_SEARCH_ID)
    try:
        driver.get(search_url)
        WebDriverWait(driver, 20).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".search-results video-tile"))
        )
        print("✅ 已到達影片搜尋頁面。")
        return True
    except Exception as e:
        print(f"❌ 導向影片搜尋頁面失敗: {e}")
        save_debug_info(driver, "goVideoPage_error")
        return False

def process_single_video(driver, loop_index, 
                         default_download_dir_for_videos, target_move_base_dir_for_videos, 
                         txt_output_base_dir, perform_video_download_flag):
    """處理單個影片的邏輯：驗證、擷取元數據、(可選)下載、儲存文字。"""
    print(f"\n🔄 正在處理第 {loop_index + 1} 部影片...")
    # 用於在擷取真實ID之前的除錯日誌，或擷取失敗時的備用ID
    default_id_for_this_loop = f"loop_{loop_index + 1}_id_unknown" 

    if not verify_page_elements_visible(driver):
        save_debug_info(driver, "page_elements_fail", default_id_for_this_loop)
        return True # 即使頁面載入失敗，也標記為“已嘗試處理”，以便主迴圈繼續

    metadata = extract_video_metadata(driver)
    video_id = metadata.get("id", default_id_for_this_loop) # 如果擷取失敗，則使用預設
    title_text = metadata.get("title", "N/A")
    
    # 確定用於本次處理和日誌的ID (優先使用有效擷取的ID)
    current_processing_log_id = video_id if not str(video_id).startswith("unknown_") else default_id_for_this_loop

    # 檢查是否已處理 (僅對有效擷取的 ID 進行檢查)
    if not str(video_id).startswith("unknown_") and not str(video_id).startswith("loop_"):
        if check_if_id_processed(video_id, base_data_path=txt_output_base_dir): # 檢查 Download/ap/{video_id}
            print(f"✅ Video ID: {video_id} 先前已處理完成 (目錄存在)，跳過此頁。")
            return True # 跳過也是一種「成功處理」
    else: 
        print(f"   -> Video ID 為 '{video_id}' (無效或預設)，將嘗試處理而不進行跳過檢查。")
    
    print(f"   -> 將使用 Video ID '{current_processing_log_id}' 進行本次處理的日誌和除錯檔案命名。")
    save_debug_info(driver, "content_and_metadata_visible", current_processing_log_id)

    if perform_video_download_flag:
        print("ℹ️ 已啟用影片下載功能。")
        if click_main_download_button(driver):
            if not str(video_id).startswith("unknown_") and not str(video_id).startswith("loop_"): # 確保 video_id 有效
                wait_for_download_and_move(video_id, default_download_dir_for_videos, target_move_base_dir_for_videos)
            else:
                print(f"   ⚠️ Video ID ({video_id}) 無效或未知，無法自動移動下載檔案。影片（如果已下載）將保留在 temp_videos。")
        else:
            print(f"   ⚠️ 主下載按鈕點擊失敗，跳過影片下載與移動。")
    else:
        print("ℹ️ 已跳過影片下載步驟。")
    
    extract_and_save_text_details(driver, video_id, title_text, txt_output_base_dir) # 使用擷取到的 video_id (可能是unknown)
    return True # 表示此影片處理嘗試已完成（無論下載成功與否，文字都會嘗試處理）

def process_videos_loop(driver, limit, perform_video_download): # 新增 perform_video_download 參數
    """主迴圈，處理多個影片。"""
    print(f"🔎 開始主迴圈處理影片，目標數量: {limit}, 是否下載影片: {'是' if perform_video_download else '否'}")
    
    # 從 init_driver 獲取預設下載位置，或在此處重新定義
    default_download_dir_for_videos = os.path.abspath(os.path.join(os.getcwd(), DOWNLOAD_BASE_DIR, AP_SUB_DIR, TEMP_VIDEOS_SUB_DIR))
    # 移動影片的目标基礎路徑 (Download/ap)
    target_move_base_dir_for_videos = os.path.abspath(os.path.join(os.getcwd(), DOWNLOAD_BASE_DIR, AP_SUB_DIR))
    # TXT 檔案儲存的基礎路徑 (Download/ap)
    txt_output_base_dir = os.path.abspath(os.path.join(os.getcwd(), DOWNLOAD_BASE_DIR, AP_SUB_DIR))


    # 導航到第一個影片
    try:
        print("⏳ 等待第一個影片連結..."); 
        first_video_link = WebDriverWait(driver, 20).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".search-results video-tile a"))
        )
        video_url = first_video_link.get_attribute('href')
        print(f"🔗 找到連結: {video_url}")
        full_video_url = urljoin(BASE_URL, video_url)
        print(f"🚗 導航至: {full_video_url}")
        driver.get(full_video_url)
        print("🧭 已導航。")
        random_sleep(1, 2)
        handle_onetrust_popup(driver)
        random_sleep(2, 3) 
    except Exception as e: 
        print(f"❌ 導航至第一個影片失敗: {e}")
        save_debug_info(driver, "initial_navigation_error")
        return # 初始導航失敗，無法繼續

    for i in range(limit):
        # 用於在 process_single_video 內部擷取 ID 前發生錯誤時的備用 ID
        video_id_for_loop_error_log = f"loop_{i+1}_before_id_extraction" 
        try:
            process_single_video(driver, i, 
                                 default_download_dir_for_videos, 
                                 target_move_base_dir_for_videos, 
                                 txt_output_base_dir,
                                 perform_video_download) # 傳遞下載標誌
            
            if i < limit - 1: # 如果不是最後一個影片
                 if not navigate_to_next_video_page(driver, i, limit): # 使用更名後的函式
                     break # 如果導航到下一頁失敗，則中止迴圈
            elif i == limit -1 : # 如果是最後一個，明確打印已達上限
                 print("🏁 已達到處理上限 (在迴圈末端)。")

        except Exception as e:
            print(f"❌ 處理第 {i + 1} 部影片時發生未預期錯誤於主迴圈: {e}")
            # 嘗試在錯誤時獲取當前影片的ID（如果可能）
            id_in_error_case = video_id_for_loop_error_log
            try: 
                metadata_in_error = extract_video_metadata(driver)
                if metadata_in_error and metadata_in_error.get("id") and not str(metadata_in_error.get("id")).startswith("unknown_"):
                    id_in_error_case = metadata_in_error.get("id")
            except Exception as inner_e:
                print(f"   -> 在主迴圈 except 區塊中嘗試擷取 metadata 失敗: {inner_e}")
            save_debug_info(driver, "loop_processing_unhandled_error", id_in_error_case)
            
            if i < limit - 1: # 即使發生錯誤，也嘗試導航到下一個
                if not navigate_to_next_video_page(driver, i, limit): # 使用更名後的函式
                    print("❌ 錯誤後嘗試恢復導航失敗，流程中止。")
                    break
            else:
                print("🏁 錯誤發生在最後一個影片處理，迴圈結束。")
                break
            
    print("✅ 所有影片處理迴圈完成。")

# --- 主程式執行區塊 ---
if __name__ == "__main__":
    driver_instance = init_driver() # 先初始化 driver

    if driver_instance:
        if login(driver_instance): # 將 driver 實例傳入 login
            
            # --- *** 新增：詢問是否下載影片 *** ---
            while True:
                user_choice = input("❓ 是否要下載影片檔案 (y/n，預設為 y)？ ").strip().lower()
                if user_choice in ['y', 'n', '']:
                    download_video_files_flag = user_choice != 'n' # 空白或 'y' 都視為 True
                    break
                else:
                    print("   無效輸入，請輸入 'y' 或 'n'。")
            # --- *** 新增結束 *** ---

            limit = 5 # 您可以修改這個數量
            try:
                if go_to_search_page(driver_instance): # 使用 driver_instance
                    process_videos_loop(driver_instance, limit, download_video_files_flag) # 傳遞下載標誌
            except Exception as e:
                print(f"❌ 主流程執行過程中發生未預期的錯誤: {e}")
                save_debug_info(driver_instance, "main_fatal_error")
            finally:
                print("\n🛑 程式執行完畢。瀏覽器將保持開啟狀態以便您查看結果或手動操作。")
                # driver_instance.quit() # 如果需要，可以取消註解以自動關閉
                pass
        else:
            print("❌ 登入失敗，程式無法繼續執行。")
            if driver_instance: # 如果 driver 已初始化但登入失敗，也將其關閉
                driver_instance.quit()
    else:
        print("❌ WebDriver 初始化失敗，程式無法執行。")