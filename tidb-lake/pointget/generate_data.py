import csv
import json
import random
import time
import uuid

# --- CONFIGURATION ---
TOTAL_ROWS_TO_GENERATE = 100000000  # 100 Million Rows
OUTPUT_FORMAT = "csv"            
OUTPUT_FILE = f"/home/admin/workspace/audience_test_data.csv" # Save it to your big 200G workspace drive!

APPID_POOL = [f"app_{i:03d}" for i in range(1, 21)]  
TAG_KEYS = ["age_group", "gender", "user_interest", "geo_region", "device_type", "churn_risk"]
TAG_VALUES = ["high", "low", "medium", "male", "female", "tier_1", "tier_2", "gaming", "shopping", "25-34", "35-44"]

def generate_data_streaming(total_rows):
    """
    Memory-safe generator that yields rows one by one.
    NO internal sets are used, keeping RAM usage flat near 0MB.
    """
    print(f"🚀 Initializing high-speed streaming of {total_rows} rows...")
    
    for generated_count in range(1, total_rows + 1):
        # Generate random unique-behaving keys using a large random space
        ctwid = f"usr_{random.randint(10000000, 99999999)}"
        appid = random.choice(APPID_POOL)
        tag_key = random.choice(TAG_KEYS)
        
        row_id = str(uuid.uuid4())[:20]  
        tag_value = random.choice(TAG_VALUES)
        updated = int(time.time() * 1000) 
        chunk = random.randint(1, 100)    
        
        if generated_count % 1000000 == 0:
            print(f"📦 Progress: {generated_count}/{total_rows} rows flushed to disk.")
            
        yield {
            "id": row_id,
            "ctwid": ctwid,
            "appid": appid,
            "tag_key": tag_key,
            "tag_value": tag_value,
            "updated": updated,
            "chunk": chunk
        }

def save_as_csv(data_generator, filename):
    fieldnames = ["id", "ctwid", "appid", "tag_key", "tag_value", "updated", "chunk"]
    with open(filename, mode="w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        for row in data_generator:
            writer.writerow(row) # Flushes directly to your NVMe disk storage line-by-line

if __name__ == "__main__":
    start_time = time.time()
    
    dataset = generate_data_streaming(TOTAL_ROWS_TO_GENERATE)
    save_as_csv(dataset, OUTPUT_FILE)
        
    duration = time.time() - start_time
    print(f"✨ Successfully wrote 100M rows in {duration:.2f} seconds!")
