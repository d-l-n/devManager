from PySide6.QtWidgets import QDialog, QVBoxLayout


class GlobalPanelDialog(QDialog):
    """Modeless holder for a project-independent panel (Monitor, App Log).

    - Opened via sidebar buttons or View menu.
    - Second invocation brings the existing dialog to front (never duplicated).
    - Closing with X only hides it; panel state is preserved.
    """

    def __init__(self, title: str, panel, parent=None):
        super().__init__(parent)
        self.setWindowTitle(title)
        self.setModal(False)
        self._panel = panel

        layout = QVBoxLayout(self)
        layout.setContentsMargins(10, 10, 10, 10)
        layout.addWidget(panel)

        self.resize(780, 520)

    def reveal(self):
        """Show/raise/activate in one call."""
        self.show()
        self.raise_()
        self.activateWindow()

    def closeEvent(self, event):
        # Hide instead of closing so panel state survives.
        event.ignore()
        self.hide()
