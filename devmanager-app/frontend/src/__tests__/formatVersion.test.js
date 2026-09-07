import { describe, it, expect } from 'vitest';
import { formatVersion } from '../api.js';

describe('formatVersion', () => {
    it('añade la v a versiones sin prefijo', () => {
        expect(formatVersion('2.0.1')).toBe('v2.0.1');
    });

    it('no duplica la v', () => {
        expect(formatVersion('v2.0.1')).toBe('v2.0.1');
    });

    it('no decora "dev" como "vdev"', () => {
        expect(formatVersion('dev')).toBe('dev');
    });

    it('defensa extra: normaliza "vdev"', () => {
        expect(formatVersion('vdev')).toBe('dev');
    });

    it('devuelve vacío para falsy', () => {
        expect(formatVersion('')).toBe('');
        expect(formatVersion(null)).toBe('');
        expect(formatVersion(undefined)).toBe('');
    });
});