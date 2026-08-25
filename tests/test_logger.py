import pytest
from app.utils.app_logger import AppLogger


def test_app_logger_singleton():
    logger1 = AppLogger.instance()
    logger2 = AppLogger.instance()
    assert logger1 is logger2


def test_app_logger_emit_and_history():
    logger = AppLogger.instance()
    received = []
    
    def on_msg(text, is_error):
        received.append((text, is_error))
        
    logger.log_message.connect(on_msg)
    
    logger.info("Test Info Message")
    logger.error("Test Error Message")
    
    assert len(received) >= 2
    assert ("Test Info Message", False) in received
    assert ("Test Error Message", True) in received
    
    history = logger.get_history()
    assert any(item[1] == "Test Info Message" and item[2] is False for item in history)
    assert any(item[1] == "Test Error Message" and item[2] is True for item in history)
