FROM python:3.6-slim

COPY requirements.txt /app/
RUN pip install -r /app/requirements.txt

COPY server.py /app/
COPY static /app/static


WORKDIR /app

CMD ["python", "/app/server.py"]
