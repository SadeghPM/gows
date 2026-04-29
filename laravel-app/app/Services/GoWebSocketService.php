<?php

namespace App\Services;

use Illuminate\Support\Facades\Http;

class GoWebSocketService
{
    /**
     * Broadcasts a JSON payload to a specific user ID via the Go WebSocket server.
     * 
     * @param int|string $userId The target user ID.
     * @param string $event The event name.
     * @param array $payload The event data/payload.
     * @return bool True if successful, false otherwise.
     */
    public static function broadcastToUser($userId, string $event, array $payload): bool
    {
        return self::sendToPlatform([
            'user_id' => (string) $userId,
            'event'   => $event,
            'payload' => $payload
        ]);
    }
    /**
     * Broadcasts a JSON payload to a specific channel via the Go WebSocket server.
     * 
     * @param string $channel The target channel name.
     * @param string $event The event name.
     * @param array $payload The event data/payload.
     * @return bool True if successful, false otherwise.
     */
    public static function broadcastToChannel(string $channel, string $event, array $payload): bool
    {
        return self::sendToPlatform([
            'channel' => $channel,
            'event' => $event,
            'payload' => $payload
        ]);
    }

    /**
     * Reusable internal method that triggers the HTTP push
     */
    private static function sendToPlatform(array $body): bool
    {
        $goBroadcastUrl = env('WS_BROADCAST_URL', 'http://localhost:8080/api/internal/broadcast');
        $internalSecret = env('WS_INTERNAL_SECRET', 'super-secret-internal-key');

        try {
            $response = Http::withHeaders([
                'Authorization' => 'Bearer ' . $internalSecret
            ])->post($goBroadcastUrl, $body);

            return $response->successful();
        } catch (\Exception $e) {
            \Log::error('Failed to broadcast to Go WS server: ' . $e->getMessage());
            return false;
        }
    }
}
