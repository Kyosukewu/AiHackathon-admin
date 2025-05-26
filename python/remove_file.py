import os
import re
import shutil

def process_downloaded_files(target_download_dir):
    print("🔄 開始處理下載檔案")
    # 定義全域下載目錄（瀏覽器預設下載目錄）
    global_download_dir = "/Users/tvbs/Downloads"
    # 確保目標資料夾存在
    os.makedirs(target_download_dir, exist_ok=True)
    
    # 定義符合條件的檔案的正規表示式： {數字}_{任意英數}_{任意英數}.mp4
    pattern = re.compile(r"^(\d+)_([A-Za-z0-9]+)_([A-Za-z0-9]+)\.mp4$")
    
    # 取得全域下載目錄中的所有檔案
    all_files = os.listdir(global_download_dir)
    matching_files = [fname for fname in all_files if pattern.match(fname)]
    
    if not matching_files:
        print("⚠️ 找不到符合條件的檔案")
        return
    
    # 依序處理所有符合格式的檔案：重新命名後搬移至目標資料夾
    for filename in matching_files:
        try:
            match = pattern.match(filename)
            if match:
                # 取出數字部分作為新檔名（依據原先的邏輯）
                new_name = f"{match.group(1)}.mp4"
                src_path = os.path.join(global_download_dir, filename)
                renamed_path = os.path.join(global_download_dir, new_name)
                
                # 重新命名檔案
                os.rename(src_path, renamed_path)
                if os.path.exists(renamed_path):
                    print(f"✅ 檔案 {filename} 已改名為 {new_name}（位於全域下載目錄）")
                else:
                    print(f"❌ 檔案 {filename} 重命名失敗")
                    continue
                
                # 搬移檔案到目標資料夾
                dst_path = os.path.join(target_download_dir, new_name)
                shutil.move(renamed_path, dst_path)
                if os.path.exists(dst_path):
                    print(f"✅ 影片成功移動到 {target_download_dir}")
                else:
                    print(f"❌ 檔案 {new_name} 移動失敗")
            else:
                print(f"❌ 檔案 {filename} 不符合格式，跳過")
        except Exception as e:
            print(f"❌ 處理檔案 {filename} 時發生錯誤: {e}")
            continue
    print("🔄 下載檔案處理完成")

if __name__ == "__main__":
    target_download_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "Download", "ap"))
    process_downloaded_files(target_download_dir)