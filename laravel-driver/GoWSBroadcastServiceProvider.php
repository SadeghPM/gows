<?php

namespace App\Providers;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Broadcast;
use Illuminate\Support\Facades\Route;
use App\Broadcasting\GoWSBroadcaster;
use App\Http\Controllers\WebSocketTicketController;

class GoWSBroadcastServiceProvider extends ServiceProvider
{
    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        // 1. Extend the Broadcast Manager to use the 'gows' driver
        Broadcast::extend('gows', function ($app, $config) {
            return new GoWSBroadcaster();
        });

        // 2. Register the routing required for ticketing
        $this->registerRoutes();
    }

    /**
     * Register the GoWS routes dynamically.
     */
    protected function registerRoutes(): void
    {
        // 1. Web Route: Called by JS client (browser) to generate a ticket.
        // Requires 'web' middleware for session/CSRF token access.
        Route::middleware('web')
            ->post('/ws/generate-ticket', [WebSocketTicketController::class, 'generateTicket'])
            ->name('gows.generate-ticket');

        // 2. Internal API Route: Called by the Go Server to validate the ticket.
        // No auth middleware because it uses a secret key in the controller.
        Route::post('/api/internal/ws/validate-ticket', [WebSocketTicketController::class, 'validateTicket'])
            ->name('gows.validate-ticket');
    }

    /**
     * Register any application services.
     */
    public function register(): void
    {
        //
    }
}
