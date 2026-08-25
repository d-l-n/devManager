# -*- coding: utf-8 -*-
import pytest
from app.ui.theme import ThemeManager, ThemeMode, THEME_CYCLE


# ---------------------------------------------------------------------------
# WCAG 2.x relative luminance / contrast ratio helpers (hex colors only)
# ---------------------------------------------------------------------------

def _rel_lum(hex_color: str) -> float:
    h = hex_color.lstrip("#")
    r, g, b = (int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4))

    def lin(v: float) -> float:
        return v / 12.92 if v <= 0.04045 else ((v + 0.055) / 1.055) ** 2.4

    r, g, b = lin(r), lin(g), lin(b)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def _contrast(fg: str, bg: str) -> float:
    l1, l2 = sorted((_rel_lum(fg), _rel_lum(bg)), reverse=True)
    return (l1 + 0.05) / (l2 + 0.05)


def test_theme_manager_singleton():
    t1 = ThemeManager.instance()
    t2 = ThemeManager.instance()
    assert t1 is t2


def test_theme_manager_toggle_cycles_three_modes():
    tm = ThemeManager.instance()
    orig = tm.mode

    expected_order = THEME_CYCLE[(THEME_CYCLE.index(orig) + 1) % len(THEME_CYCLE)]
    new_mode = tm.toggle_theme()
    assert new_mode == expected_order

    restored = tm.toggle_theme()
    assert restored != new_mode

    # Full cycle returns to the original mode.
    tm.toggle_theme()
    assert tm.mode == orig


def test_is_dark_covers_dark_family():
    tm = ThemeManager.instance()
    assert tm.get_colors(ThemeMode.DARK)["bg_base"] != "#000000"
    # is_dark must be True for OLED too so all dark-family branches keep working.
    original = tm.mode
    tm.set_theme(ThemeMode.DARK)
    assert tm.is_dark and not tm.is_oled
    tm.set_theme(ThemeMode.OLED)
    assert tm.is_dark and tm.is_oled
    tm.set_theme(ThemeMode.LIGHT)
    assert not tm.is_dark and not tm.is_oled
    tm.set_theme(original)


def test_theme_colors():
    tm = ThemeManager.instance()
    dark_colors = tm.get_colors(ThemeMode.DARK)
    light_colors = tm.get_colors(ThemeMode.LIGHT)

    assert "bg_base" in dark_colors
    assert "bg_base" in light_colors
    assert dark_colors["bg_base"] != light_colors["bg_base"]
    assert dark_colors["primary"] != ""
    assert light_colors["primary"] != ""


def test_oled_palette_is_true_black_and_complete():
    tm = ThemeManager.instance()
    oled = tm.get_colors(ThemeMode.OLED)
    dark = tm.get_colors(ThemeMode.DARK)

    # Same color roles as the other palettes (no widget left without a color).
    assert set(oled.keys()) == set(dark.keys())
    # OLED hallmark: pure black base and terminal.
    assert oled["bg_base"] == "#000000"
    assert oled["terminal_bg"] == "#000000"
    # Distinct from standard DARK palette.
    assert oled["bg_surface"] != dark["bg_surface"]
    # Surfaces stay darker than text for hierarchy.
    assert oled["bg_elevated"] != oled["bg_base"]


@pytest.mark.parametrize("mode", list(ThemeMode))
@pytest.mark.parametrize("surface", ["bg_base", "bg_surface", "bg_card"])
def test_body_text_contrast_all_modes(mode, surface):
    """Body/secondary text must stay readable on every background in every mode."""
    c = ThemeManager.instance().get_colors(mode)
    bg = c[surface]
    assert _contrast(c["text_primary"], bg) >= 7.0     # AAA body text
    assert _contrast(c["text_secondary"], bg) >= 4.5   # AA body text
    assert _contrast(c["text_muted"], bg) >= 3.5       # AA large text / hints
    assert _contrast(c["primary"], bg) >= 3.5          # selected/tab labels


@pytest.mark.parametrize("mode", list(ThemeMode))
def test_accent_and_terminal_contrast_all_modes(mode):
    c = ThemeManager.instance().get_colors(mode)
    # Status accents are used as text on base backgrounds and in the terminal.
    for accent in ("success", "warning", "danger", "info"):
        assert _contrast(c[accent], c["bg_base"]) >= 2.8
        assert _contrast(c[accent], c["terminal_bg"]) >= 4.0
    # Terminal text itself.
    assert _contrast(c["terminal_fg"], c["terminal_bg"]) >= 7.0
    # Icons are decorative graphics: 3:1 minimum.
    assert _contrast(c["icon_color"], c["bg_base"]) >= 3.0


def test_oled_meets_full_wcag_aa():
    """OLED gets the strictest bar: every text role hits 4.5:1 on its surfaces.

    Pure black backgrounds cause halation; grays tuned for regular dark themes
    become unreadable. These guards pin that legibility does not regress.
    """
    c = ThemeManager.instance().get_colors(ThemeMode.OLED)
    for surface in ("bg_base", "bg_surface", "bg_card", "bg_elevated"):
        bg = c[surface]
        assert _contrast(c["text_primary"], bg) >= 10.0
        assert _contrast(c["text_secondary"], bg) >= 4.5
        assert _contrast(c["text_muted"], bg) >= 4.5
        assert _contrast(c["primary"], bg) >= 4.5
    for accent in ("success", "warning", "danger", "info"):
        assert _contrast(accent_color := c[accent], c["terminal_bg"]) >= 6.0


def test_theme_stylesheets():
    tm = ThemeManager.instance()
    dark_qss = tm.get_main_stylesheet(ThemeMode.DARK)
    light_qss = tm.get_main_stylesheet(ThemeMode.LIGHT)

    assert "QMainWindow" in dark_qss
    assert "QMainWindow" in light_qss
    assert dark_qss != light_qss

    dark_dialog_qss = tm.get_dialog_stylesheet(ThemeMode.DARK)
    light_dialog_qss = tm.get_dialog_stylesheet(ThemeMode.LIGHT)
    assert "QDialog" in dark_dialog_qss
    assert "QDialog" in light_dialog_qss

    # OLED reuses the same stylesheets with its own palette injected.
    oled_qss = tm.get_main_stylesheet(ThemeMode.OLED)
    assert "QMainWindow" in oled_qss
    assert "#000000" in oled_qss
    assert oled_qss != dark_qss

    oled_dialog_qss = tm.get_dialog_stylesheet(ThemeMode.OLED)
    assert "QDialog" in oled_dialog_qss
