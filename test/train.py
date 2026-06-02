import time

start = time.time()
while time.time() - start < 30:
    print("this is train script", flush=True)
    time.sleep(3)
