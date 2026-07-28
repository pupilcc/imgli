# imgli OCR sidecar — PaddleOCR (paddlepaddle 3.x; 2.6 segfaults on some Zen5 hosts)
FROM python:3.10-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    FLAGS_use_mkldnn=0 \
    FLAGS_onednn=0 \
    FLAGS_enable_pir_api=0 \
    FLAGS_enable_pir_in_executor=0 \
    FLAGS_enable_new_ir_in_executor=0 \
    OMP_NUM_THREADS=2 \
    PORT=3199 \
    HOST=0.0.0.0 \
    OCR_LANG=ch \
    OCR_USE_ANGLE_CLS=1

RUN apt-get update -qq \
    && apt-get install -y -qq --no-install-recommends \
        libgomp1 libgl1 libglib2.0-0 libsm6 libxext6 libxrender1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
        "paddlepaddle==3.3.1" \
        paddleocr \
        opencv-python-headless \
        pillow

WORKDIR /app
COPY server.py /app/server.py

# Pre-download models
RUN python3 -c "from paddleocr import PaddleOCR; PaddleOCR(lang='ch', use_textline_orientation=True); print('models_ok')"

EXPOSE 3199
CMD ["python3", "/app/server.py"]
