<?php

namespace App\Broadcasting;

use Illuminate\Contracts\Broadcasting\Broadcaster;
use App\Services\GoWebSocketService;
use Symfony\Component\HttpKernel\Exception\AccessDeniedHttpException;

class GoWSBroadcaster implements Broadcaster
{
    /**
     * Authenticate the incoming request for a given channel.
     *
     * Note: GoWS client (gows.js) does not perform standard Pusher-style
     * channel authentication requests. It authenticates upfront via a ticket.
     * Therefore, this method is rarely hit, but we implement it to satisfy the interface.
     *
     * @param  \Illuminate\Http\Request  $request
     * @return mixed
     */
    public function auth($request)
    {
        // GoWS establishes auth upfront. Standard channel auth isn't used by gows.js.
        return true;
    }

    /**
     * Return the valid authentication response.
     *
     * @param  \Illuminate\Http\Request  $request
     * @param  mixed  $result
     * @return mixed
     */
    public function validAuthenticationResponse($request, $result)
    {
        return $result;
    }

    /**
     * Broadcast the given event.
     *
     * @param  array  $channels
     * @param  string  $event
     * @param  array  $payload
     * @return void
     */
    public function broadcast(array $channels, $event, array $payload = [])
    {
        foreach ($channels as $channel) {
            $this->broadcastToChannel($channel, $event, $payload);
        }
    }

    /**
     * Route the broadcast to the correct GoWS Service method based on channel type.
     */
    protected function broadcastToChannel($channel, $event, array $payload)
    {
        // Handle Private User Channels (e.g. private-App.Models.User.1)
        if (str_starts_with($channel, 'private-App.Models.User.')) {
            $userId = str_replace('private-App.Models.User.', '', $channel);
            GoWebSocketService::broadcastToUser($userId, $event, $payload);
            return;
        }

        // Handle standard private channels (if gows.js is adapted to use them)
        if (str_starts_with($channel, 'private-')) {
            $cleanChannel = str_replace('private-', '', $channel);
            GoWebSocketService::broadcastToChannel($cleanChannel, $event, $payload);
            return;
        }

        // Handle presence channels (if gows.js is adapted to use them)
        if (str_starts_with($channel, 'presence-')) {
            $cleanChannel = str_replace('presence-', '', $channel);
            GoWebSocketService::broadcastToChannel($cleanChannel, $event, $payload);
            return;
        }

        // Standard public channel fallback
        GoWebSocketService::broadcastToChannel($channel, $event, $payload);
    }
}
