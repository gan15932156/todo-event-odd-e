import os
import json
import time
import signal
import logging
import requests
import pika
from threading import Event

# ----------------------------
# Logging setup
# ----------------------------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s"
)

# ----------------------------
# Constants
# ----------------------------
TASK_EXCHANGE = "task.exchange"
ONBOARDING_EXCHANGE = "onboarding.exchange"

QUEUE_AUDIT_TASK_EVENTS = "audit.task.events"
QUEUE_AUDIT_USER_EVENTS = "audit.user.events"

# ----------------------------
# Loki Push
# ----------------------------
def push_to_loki(loki_url: str, event_type: str, payload: dict):
    line = json.dumps({
        "event_type": event_type,
        "payload": payload
    })

    body = {
        "streams": [
            {
                "stream": {
                    "service": "audit",
                    "event_type": event_type
                },
                "values": [
                    [
                        str(time.time_ns()),
                        line
                    ]
                ]
            }
        ]
    }

    url = f"{loki_url}/loki/api/v1/push"

    response = requests.post(url, json=body)

    if response.status_code >= 300:
        raise Exception(f"loki push failed: {response.status_code}")


# ----------------------------
# RabbitMQ Connect
# ----------------------------
def connect_rabbitmq(amqp_url):
    params = pika.URLParameters(amqp_url)
    connection = pika.BlockingConnection(params)
    channel = connection.channel()
    return connection, channel


# ----------------------------
# Subscribe Function
# ----------------------------
def subscribe(channel, exchange_name, queue_name):
    channel.exchange_declare(exchange=exchange_name, exchange_type="fanout", durable=True)
    channel.queue_declare(queue=queue_name, durable=True)
    channel.queue_bind(exchange=exchange_name, queue=queue_name)


# ----------------------------
# Message Handler
# ----------------------------
def create_callback(loki_url):
    def callback(ch, method, properties, body):
        try:
            msg = json.loads(body.decode())

            event_type = msg.get("type", "unknown")
            payload = msg.get("payload", {})

            logging.info(f"audit: received event type={event_type}")

            try:
                push_to_loki(loki_url, event_type, payload)
            except Exception as e:
                logging.error(f"audit: loki push error {e}")

            ch.basic_ack(delivery_tag=method.delivery_tag)

        except Exception as e:
            logging.error(f"message processing error: {e}")

    return callback


# ----------------------------
# Main
# ----------------------------
def main():
    amqp_url = os.getenv("AMQP_URL", "amqp://guest:guest@localhost:5672/")
    loki_url = os.getenv("LOKI_URL", "http://localhost:3100")

    connection, channel = connect_rabbitmq(amqp_url)

    subscribe(channel, TASK_EXCHANGE, QUEUE_AUDIT_TASK_EVENTS)
    subscribe(channel, ONBOARDING_EXCHANGE, QUEUE_AUDIT_USER_EVENTS)

    callback = create_callback(loki_url)

    channel.basic_consume(
        queue=QUEUE_AUDIT_TASK_EVENTS,
        on_message_callback=callback
    )

    channel.basic_consume(
        queue=QUEUE_AUDIT_USER_EVENTS,
        on_message_callback=callback
    )

    logging.info("audit service listening")

    stop_event = Event()

    def shutdown_handler(signum, frame):
        logging.info("audit service stopping")
        stop_event.set()
        channel.stop_consuming()

    signal.signal(signal.SIGINT, shutdown_handler)
    signal.signal(signal.SIGTERM, shutdown_handler)

    try:
        channel.start_consuming()
    except KeyboardInterrupt:
        shutdown_handler(None, None)

    connection.close()


if __name__ == "__main__":
    main()