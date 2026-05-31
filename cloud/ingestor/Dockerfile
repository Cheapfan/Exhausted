FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY db/ ./db/
COPY ingestor/ ./ingestor/

EXPOSE 8000

CMD ["uvicorn", "ingestor.main:app", "--host", "0.0.0.0", "--port", "8000"]
