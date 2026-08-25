import sys
import os
from PySide6.QtWidgets import QApplication
from app.utils.app_logger import AppLogger
from app.config.manager import ConfigManager
from app.ui.theme import ui_font
from app.ui.main_window import MainWindow

if __name__ == '__main__':
    config_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'projects.json')
    app = QApplication(sys.argv)

    # Install the app logger early so all stdout/stderr is captured
    app_logger = AppLogger.instance()
    app_logger.install()
    app_logger.info("Local Dev Manager starting...")

    app.setFont(ui_font())

    config_manager = ConfigManager(config_path)
    app_logger.info(f"Loaded {config_manager.project_count()} project(s) from config")

    window = MainWindow(config_manager)
    window.show()

    app_logger.info("Application ready")
    sys.exit(app.exec())
