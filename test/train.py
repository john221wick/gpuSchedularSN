import time
import sys
import os

epochs = int(sys.argv[1]) if len(sys.argv) > 1 else 10
batch_size = int(sys.argv[2]) if len(sys.argv) > 2 else 32

gpu_ids = os.environ.get("CUDA_VISIBLE_DEVICES", os.environ.get("HIP_VISIBLE_DEVICES", "none"))
print(f"Training on GPUs: {gpu_ids}")
print(f"Epochs: {epochs}, Batch size: {batch_size}")
print()

for epoch in range(1, epochs + 1):
    steps = 5
    for step in range(1, steps + 1):
        time.sleep(1)
        loss = 2.5 / (epoch * 0.8 + step * 0.1)
        acc = min(0.99, 0.3 + epoch * 0.06 + step * 0.01)
        print(f"Epoch {epoch}/{epochs} - Step {step}/{steps} - loss: {loss:.4f} - acc: {acc:.4f}")
    print(f"Epoch {epoch} complete\n")

print("Training finished.")
